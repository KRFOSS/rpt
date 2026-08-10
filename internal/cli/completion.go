package cli

import (
	"fmt"

	"github.com/krfoss/rpt/internal/ui"
)

// bashCompletion은 bash 자동완성 스크립트입니다.
//
// 패키지 이름은 rpt list 를 그때그때 불러 채우므로 저장소 목록이 바뀌어도
// 스크립트를 다시 깔 필요가 없습니다.
const bashCompletion = `# rpt 자동완성 (bash)
_rpt_completion() {
    local cur prev cmd
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    cmd="${COMP_WORDS[1]}"

    if [ "${COMP_CWORD}" -eq 1 ]; then
        COMPREPLY=($(compgen -W "update install remove purge upgrade autoremove list search show clean autoclean relink completion version help" -- "${cur}"))
        return
    fi

    if [ "${prev}" = "--root" ]; then
        COMPREPLY=($(compgen -d -- "${cur}"))
        return
    fi

    if [ "${cur:0:1}" = "-" ]; then
        case "${cmd}" in
            list)            COMPREPLY=($(compgen -W "--installed --upgradable --root" -- "${cur}")) ;;
            install|upgrade) COMPREPLY=($(compgen -W "-y --yes --reinstall --root" -- "${cur}")) ;;
            *)               COMPREPLY=($(compgen -W "-y --yes --root" -- "${cur}")) ;;
        esac
        return
    fi

    case "${cmd}" in
        install|show|info|search)
            COMPREPLY=($(compgen -W "$(rpt list 2>/dev/null | cut -d/ -f1)" -- "${cur}"))
            ;;
        remove|purge|upgrade)
            COMPREPLY=($(compgen -W "$(rpt list --installed 2>/dev/null | cut -d/ -f1)" -- "${cur}"))
            ;;
        completion)
            COMPREPLY=($(compgen -W "bash zsh" -- "${cur}"))
            ;;
    esac
}
complete -F _rpt_completion rpt
`

// zshCompletion은 zsh 자동완성 스크립트입니다.
const zshCompletion = `#compdef rpt
# rpt 자동완성 (zsh)

_rpt_packages_all() {
    local -a pkgs
    pkgs=(${(f)"$(rpt list 2>/dev/null | cut -d/ -f1)"})
    _describe '패키지' pkgs
}

_rpt_packages_installed() {
    local -a pkgs
    pkgs=(${(f)"$(rpt list --installed 2>/dev/null | cut -d/ -f1)"})
    _describe '설치된 패키지' pkgs
}

_rpt() {
    local -a cmds
    cmds=(
        'update:저장소 패키지 목록을 갱신합니다'
        'install:패키지와 의존성을 설치합니다'
        'remove:패키지를 지웁니다 (설정 파일은 남깁니다)'
        'purge:패키지를 설정 파일까지 지웁니다'
        'upgrade:설치된 패키지를 최신 버전으로 올립니다'
        'autoremove:딸려왔다가 필요 없어진 패키지를 지웁니다'
        'list:패키지 목록을 봅니다'
        'search:이름과 설명에서 찾습니다'
        'show:패키지 상세 정보를 봅니다'
        'clean:내려받아 둔 deb 를 모두 지웁니다'
        'autoclean:저장소에서 사라진 옛 deb 만 지웁니다'
        'relink:시스템 심링크를 다시 만듭니다'
        'completion:셸 자동완성 스크립트를 출력합니다'
        'version:rpt 버전을 봅니다'
        'help:사용법을 봅니다'
    )

    if (( CURRENT == 2 )); then
        _describe '명령' cmds
        return
    fi

    if [[ ${words[CURRENT-1]} == --root ]]; then
        _files -/
        return
    fi

    case ${words[2]} in
        install|show|info|search)
            _rpt_packages_all
            ;;
        remove|purge|upgrade|autoremove)
            _rpt_packages_installed
            ;;
        list)
            _values '옵션' '--installed[설치된 것만 봅니다]' '--upgradable[올릴 수 있는 것만 봅니다]'
            ;;
        completion)
            _values '셸' bash zsh
            ;;
    esac
}

if [ "$funcstack[1]" = "_rpt" ]; then
    _rpt "$@"
else
    compdef _rpt rpt
fi
`

// cmdCompletion은 셸 자동완성 스크립트를 출력합니다.
func cmdCompletion(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("셸 이름이 필요합니다 (bash 또는 zsh)")
	}
	switch args[0] {
	case "bash":
		ui.Out("%s", bashCompletion)
	case "zsh":
		ui.Out("%s", zshCompletion)
	default:
		return fmt.Errorf("지원하지 않는 셸입니다: %s (bash 또는 zsh)", args[0])
	}
	return nil
}
