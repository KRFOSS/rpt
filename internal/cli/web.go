package cli

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"syscall"

	"github.com/krfoss/rpt/internal/pkgmgr"
	"github.com/krfoss/rpt/internal/repo"
	webui "github.com/krfoss/rpt/internal/web"
	"github.com/krfoss/rpt/internal/ui"
)

const webForegroundEnv = "RPT_WEB_FOREGROUND"

// cmdWeb은 로컬 웹 대시보드를 실행합니다.
func cmdWeb(m *pkgmgr.Manager, opts options) error {
	addr := m.Cfg.WebAddr
	if opts.Addr != "" {
		addr = opts.Addr
	}
	if err := m.LoadIndex(); err != nil && !errors.Is(err, repo.ErrNoIndex) {
		return err
	}
	if os.Getenv(webForegroundEnv) == "" {
		return startWebBackground(addr)
	}
	ui.Step("웹 대시보드를 시작합니다: http://%s", addr)
	ui.Detail("상태 API: http://%s/api/status", addr)
	srv := webui.New(m, addr)
	err := srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("웹 서버 시작 실패: %w", err)
	}
	return nil
}

func startWebBackground(addr string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("웹 서버 실행 파일을 찾지 못했습니다: %w", err)
	}
	args := []string{exe, "web"}
	env := append(os.Environ(), webForegroundEnv+"=1")
	attr := &syscall.SysProcAttr{Setsid: true}
	proc, err := os.StartProcess(exe, args, &os.ProcAttr{
		Dir:   "",
		Env:   env,
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		Sys:   attr,
	})
	if err != nil {
		return fmt.Errorf("웹 서버를 백그라운드로 시작하지 못했습니다: %w", err)
	}
	ui.Step("웹 대시보드를 백그라운드로 시작했습니다: http://%s", addr)
	ui.Detail("프로세스 ID: %d", proc.Pid)
	ui.Detail("상태 API: http://%s/api/status", addr)
	return nil
}