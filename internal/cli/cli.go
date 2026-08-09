// Package cli는 rpt의 명령행 처리를 담당합니다.
//
// 명령 이름과 옵션은 apt와 최대한 같게 맞췄습니다. apt를 쓰던 손이
// 그대로 통하는 것이 이 도구의 목적 중 하나입니다.
package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/krfoss/rpt/internal/config"
	"github.com/krfoss/rpt/internal/pkgmgr"
	"github.com/krfoss/rpt/internal/repo"
	"github.com/krfoss/rpt/internal/ui"
	"golang.org/x/term"
)

// options는 명령행에서 뽑아낸 공통 옵션입니다.
type options struct {
	// Yes는 확인 질문을 건너뜁니다. apt의 -y 와 같습니다.
	Yes bool
	// Reinstall은 같은 버전을 다시 설치합니다.
	Reinstall bool
	// Installed는 list에서 설치된 것만 보여 줍니다.
	Installed bool
	// Upgradable은 list에서 올릴 수 있는 것만 보여 줍니다.
	Upgradable bool
	// Root는 설치 루트를 덮어씁니다.
	Root string
}

// Run은 명령행 인자를 처리하고 종료 코드를 돌려줍니다.
func Run(args []string) int {
	if len(args) == 0 {
		printUsage()
		return 1
	}

	cmd := args[0]
	rest, opts, err := parseArgs(args[1:])
	if err != nil {
		ui.Error("%v", err)
		return 1
	}
	if opts.Root != "" {
		os.Setenv("RPT_ROOT", opts.Root)
	}

	switch cmd {
	case "help", "--help", "-h":
		printUsage()
		return 0
	case "version", "--version", "-V":
		ui.Out("rpt %s (ROKFOSS 패키지 관리자, %s)", config.Version, config.DebArch())
		return 0
	}

	cfg, err := config.Load()
	if err != nil {
		ui.Error("%v", err)
		return 1
	}
	m, err := pkgmgr.New(cfg)
	if err != nil {
		ui.Error("%v", err)
		return 1
	}

	if err := dispatch(m, cmd, rest, opts); err != nil {
		if errors.Is(err, errCancelled) {
			ui.Info("작업을 취소했습니다.")
			return 1
		}
		if errors.Is(err, repo.ErrNoIndex) {
			ui.Error("패키지 목록이 없습니다. 먼저 rpt update 를 실행하십시오.")
			return 1
		}
		ui.Error("%v", err)
		return 1
	}
	return 0
}

// dispatch는 하위 명령을 알맞은 처리기로 넘깁니다.
func dispatch(m *pkgmgr.Manager, cmd string, args []string, opts options) error {
	switch cmd {
	case "update":
		return cmdUpdate(m)
	case "install":
		return cmdInstall(m, args, opts)
	case "remove":
		return cmdRemove(m, args, opts, false)
	case "purge":
		return cmdRemove(m, args, opts, true)
	case "upgrade":
		return cmdUpgrade(m, args, opts)
	case "autoremove":
		return cmdAutoremove(m, opts)
	case "list":
		return cmdList(m, opts)
	case "search":
		return cmdSearch(m, args)
	case "show", "info":
		return cmdShow(m, args)
	case "clean":
		return cmdClean(m, false)
	case "autoclean":
		return cmdClean(m, true)
	case "relink":
		return cmdRelink(m)
	default:
		return fmt.Errorf("알 수 없는 명령입니다: %s (rpt help 로 사용법을 확인하십시오)", cmd)
	}
}

// parseArgs는 옵션과 인자를 분리합니다.
// apt처럼 옵션이 인자 앞뒤 어디에 오든 받아들입니다.
func parseArgs(args []string) ([]string, options, error) {
	var (
		rest []string
		opts options
	)
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-y", "--yes", "--assume-yes":
			opts.Yes = true
		case "--reinstall":
			opts.Reinstall = true
		case "--installed":
			opts.Installed = true
		case "--upgradable", "--upgradeable":
			opts.Upgradable = true
		case "--root":
			if i+1 >= len(args) {
				return nil, opts, fmt.Errorf("--root 뒤에 경로가 필요합니다")
			}
			i++
			opts.Root = args[i]
		default:
			if strings.HasPrefix(a, "--root=") {
				opts.Root = strings.TrimPrefix(a, "--root=")
				continue
			}
			if strings.HasPrefix(a, "-") {
				return nil, opts, fmt.Errorf("알 수 없는 옵션입니다: %s", a)
			}
			rest = append(rest, a)
		}
	}
	return rest, opts, nil
}

// errCancelled는 사용자가 확인 질문에서 거부했다는 뜻입니다.
var errCancelled = errors.New("사용자가 취소했습니다")

// confirm은 apt와 같은 형식으로 진행 여부를 묻습니다.
func confirm(yes bool) error {
	if yes {
		return nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("확인을 받을 수 없습니다. 자동으로 진행하려면 -y 를 붙이십시오")
	}
	fmt.Fprint(os.Stderr, "계속하시겠습니까? [Y/n] ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return errCancelled
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "", "y", "yes", "ㅛ":
		return nil
	default:
		return errCancelled
	}
}

// requireRoot는 시스템을 고치는 명령에서 root 권한을 확인합니다.
func requireRoot() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("이 명령은 root 권한이 필요합니다 (sudo 로 다시 실행하십시오)")
	}
	return nil
}

// printUsage는 사용법을 출력합니다.
func printUsage() {
	fmt.Fprint(os.Stderr, `rpt - ROKFOSS 패키지 관리자 (`+config.Version+`)

apt 가 없는 시놀로지 DSM 등에서 ROKFOSS 저장소의 패키지를 관리합니다.
시스템 패키지 관리자의 목록과 상태를 건드리지 않습니다.

사용법:
  rpt <명령> [패키지...] [옵션]

명령:
  update              저장소 패키지 목록을 갱신합니다
  install <패키지...>  패키지와 의존성을 설치합니다
  remove <패키지...>   패키지를 지웁니다 (설정 파일은 남깁니다)
  purge <패키지...>    패키지를 설정 파일까지 지웁니다
  upgrade [패키지...]  설치된 패키지를 최신 버전으로 올립니다
  autoremove          의존성으로 딸려왔다가 필요 없어진 패키지를 지웁니다
  list                패키지 목록을 봅니다
  search <검색어>      이름과 설명에서 검색합니다
  show <패키지>        패키지 상세 정보를 봅니다
  clean               내려받아 둔 deb 파일을 모두 지웁니다
  autoclean           저장소에서 사라진 옛 deb 파일만 지웁니다
  relink              시스템 심링크를 다시 만듭니다 (DSM 업데이트 후)
  version             rpt 버전을 봅니다

옵션:
  -y, --yes           확인 질문 없이 진행합니다
      --reinstall     같은 버전이어도 다시 설치합니다
      --installed     list 에서 설치된 것만 봅니다
      --upgradable    list 에서 올릴 수 있는 것만 봅니다
      --root <경로>    설치 루트를 지정합니다 (기본: 첫 볼륨의 @rokfoss)

환경 변수:
  RPT_ROOT            설치 루트
  RPT_REPO            저장소 주소 (기본: `+config.DefaultRepoURL+`)
  RPT_BINDIR          실행 파일 심링크 위치 (기본: `+config.DefaultBinDir+`)
  RPT_ETCDIR          설정 디렉터리 심링크 위치 (기본: `+config.DefaultEtcDir+`)

예시:
  rpt update
  rpt install krfs-rport -y
  rpt list --installed
  rpt remove krfs-rport
  rpt clean
`)
}
