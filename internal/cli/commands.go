package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/krfoss/rpt/internal/deb"
	"github.com/krfoss/rpt/internal/pkgmgr"
	"github.com/krfoss/rpt/internal/repo"
	"github.com/krfoss/rpt/internal/state"
	"github.com/krfoss/rpt/internal/ui"
)

// cmdUpdate는 저장소 목록을 새로 받아 검증합니다.
func cmdUpdate(m *pkgmgr.Manager) error {
	ui.Step("저장소 목록을 갱신합니다: %s", m.Cfg.RepoURL)
	ix, err := m.Client.Update()
	if err != nil {
		return err
	}
	if ix.Signer != nil {
		ui.Detail("서명 확인: %s", ix.Signer.Identity)
	}
	ui.Detail("%s / %s / %s", ix.Origin, ix.Suite, ix.Component)
	if len(ix.OtherArches) > 0 {
		ui.Info("%s 패키지 %d개 (%s 목록은 건너뜁니다)",
			ix.Arch, len(ix.Packages), strings.Join(ix.OtherArches, ", "))
	} else {
		ui.Info("%s 패키지 %d개", ix.Arch, len(ix.Packages))
	}
	return nil
}

// cmdInstall은 패키지를 설치합니다.
func cmdInstall(m *pkgmgr.Manager, names []string, opts options) error {
	if len(names) == 0 {
		return fmt.Errorf("설치할 패키지 이름이 필요합니다 (예: rpt install krfs-rport)")
	}
	if err := requireRoot(); err != nil {
		return err
	}
	if err := m.LoadIndex(); err != nil {
		return err
	}

	plan, err := m.PlanInstall(names, opts.Reinstall)
	if err != nil {
		return err
	}
	for _, n := range plan.AlreadyCurrent {
		ui.Info("%s 는 이미 최신 버전입니다.", n)
	}
	for _, d := range plan.MissingDeps {
		ui.Warn("의존성 %s 는 ROKFOSS 저장소에 없습니다. 시스템에 이미 있는지 확인하십시오.", d)
	}
	if plan.Empty() {
		ui.Info("할 일이 없습니다.")
		return nil
	}

	printInstallPlan(plan)
	if err := confirm(opts.Yes); err != nil {
		return err
	}

	results, err := m.Execute(plan, func(i, total int, item pkgmgr.PlanItem) {
		ui.Step("[%d/%d] %s %s (%s)", i, total, item.Pkg.Name, item.Pkg.Version, item.Action)
	})
	if err != nil {
		return err
	}

	printInstallNotes(m, results)
	return nil
}

// printInstallPlan은 apt 형식으로 작업 계획을 보여 줍니다.
func printInstallPlan(plan *pkgmgr.Plan) {
	var newly, upgraded, again []string
	for _, it := range plan.Items {
		label := fmt.Sprintf("%s (%s)", it.Pkg.Name, it.Pkg.Version)
		switch it.Action {
		case pkgmgr.ActionUpgrade:
			upgraded = append(upgraded, fmt.Sprintf("%s (%s -> %s)", it.Pkg.Name, it.Current, it.Pkg.Version))
		case pkgmgr.ActionReinstall:
			again = append(again, label)
		default:
			if it.Auto {
				label += " [의존성]"
			}
			newly = append(newly, label)
		}
	}
	section := func(title string, items []string) {
		if len(items) == 0 {
			return
		}
		ui.Info("%s", title)
		ui.Info("  %s", strings.Join(items, ", "))
	}
	section("다음 패키지를 새로 설치합니다:", newly)
	section("다음 패키지를 업그레이드합니다:", upgraded)
	section("다음 패키지를 다시 설치합니다:", again)
	ui.Info("설치 %d개, 업그레이드 %d개, 재설치 %d개 / 내려받을 용량 %s",
		len(newly), len(upgraded), len(again), ui.HumanSize(plan.DownloadSize))
}

// printInstallNotes는 설치 후 사용자가 알아야 할 것들을 정리해 보여 줍니다.
func printInstallNotes(m *pkgmgr.Manager, results []pkgmgr.InstallResult) {
	var linked []string
	for _, r := range results {
		for _, l := range r.Links {
			linked = append(linked, l.Path)
		}
		for _, p := range r.SkippedLinks {
			ui.Warn("%s 는 이미 다른 파일이 있어 링크를 만들지 않았습니다: %s", r.Name, p)
		}
		for _, c := range r.KeptConffiles {
			ui.Info("설정 파일을 그대로 두었습니다: %s", filepath.Join(m.Cfg.Root, c))
		}
		if len(r.SkippedScripts) > 0 {
			ui.Info("%s 의 관리자 스크립트(%s)는 실행하지 않았습니다.",
				r.Name, strings.Join(r.SkippedScripts, ", "))
		}
	}

	for _, r := range results {
		rec, ok := m.DB.Get(r.Name)
		if !ok {
			continue
		}
		for _, f := range rec.Files {
			if strings.Contains(f, "systemd/system/") && strings.HasSuffix(f, ".service") {
				ui.Info("서비스 파일이 있습니다: %s", filepath.Join(m.Cfg.Root, f))
				ui.Detail("rpt는 서비스를 자동 등록하지 않습니다. 필요하면 직접 등록하십시오.")
			}
		}
	}

	if len(linked) > 0 {
		sort.Strings(linked)
		ui.Info("실행 파일 링크: %s", strings.Join(linked, ", "))
		if !inPath(m.Cfg.BinDir) {
			ui.Warn("%s 가 PATH에 없습니다. PATH에 추가해야 명령을 바로 쓸 수 있습니다.", m.Cfg.BinDir)
		}
	}
	ui.Info("설치 위치: %s", m.Cfg.Root)
}

// inPath는 디렉터리가 현재 PATH에 들어 있는지 확인합니다.
func inPath(dir string) bool {
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if filepath.Clean(p) == filepath.Clean(dir) {
			return true
		}
	}
	return false
}

// cmdRemove는 패키지를 지웁니다. purge가 참이면 설정 파일까지 지웁니다.
func cmdRemove(m *pkgmgr.Manager, names []string, opts options, purge bool) error {
	if len(names) == 0 {
		return fmt.Errorf("지울 패키지 이름이 필요합니다 (예: rpt remove krfs-rport)")
	}
	if err := requireRoot(); err != nil {
		return err
	}

	plan := m.PlanRemove(names, purge)
	for _, n := range plan.NotInstalled {
		ui.Info("%s 는 설치되어 있지 않습니다.", n)
	}
	for name, blockers := range plan.Blocked {
		ui.Warn("%s 는 %s 가 의존하고 있어 지우지 않습니다.", name, strings.Join(blockers, ", "))
	}
	if plan.Empty() {
		ui.Info("할 일이 없습니다.")
		return nil
	}

	var labels []string
	for _, rec := range plan.Items {
		labels = append(labels, fmt.Sprintf("%s (%s)", rec.Name, rec.Version))
	}
	verb := "지웁니다"
	if purge {
		verb = "설정 파일까지 지웁니다"
	}
	ui.Info("다음 패키지를 %s:", verb)
	ui.Info("  %s", strings.Join(labels, ", "))
	if err := confirm(opts.Yes); err != nil {
		return err
	}

	results, err := m.ExecuteRemove(plan, func(i, total int, rec *state.Installed) {
		ui.Step("[%d/%d] %s 제거", i, total, rec.Name)
	})
	if err != nil {
		return err
	}
	for _, r := range results {
		ui.Detail("%s: 파일 %d개 삭제, 링크 %d개 해제", r.Name, r.RemovedFiles, len(r.RemovedLinks))
		for _, c := range r.KeptConffiles {
			ui.Info("설정 파일을 남겼습니다: %s (purge 로 함께 지울 수 있습니다)",
				filepath.Join(m.Cfg.Root, c))
		}
	}
	return nil
}

// cmdUpgrade는 설치된 패키지를 최신으로 올립니다.
func cmdUpgrade(m *pkgmgr.Manager, names []string, opts options) error {
	if err := requireRoot(); err != nil {
		return err
	}
	if err := m.LoadIndex(); err != nil {
		return err
	}
	plan, err := m.PlanUpgrade(names)
	if err != nil {
		return err
	}
	if plan.Empty() {
		ui.Info("모든 패키지가 최신입니다.")
		return nil
	}
	printInstallPlan(plan)
	if err := confirm(opts.Yes); err != nil {
		return err
	}
	results, err := m.Execute(plan, func(i, total int, item pkgmgr.PlanItem) {
		ui.Step("[%d/%d] %s %s (%s)", i, total, item.Pkg.Name, item.Pkg.Version, item.Action)
	})
	if err != nil {
		return err
	}
	printInstallNotes(m, results)
	return nil
}

// cmdAutoremove는 필요 없어진 자동 설치 패키지를 지웁니다.
func cmdAutoremove(m *pkgmgr.Manager, opts options) error {
	if err := requireRoot(); err != nil {
		return err
	}
	names := m.AutoRemovable()
	if len(names) == 0 {
		ui.Info("지울 것이 없습니다.")
		return nil
	}
	return cmdRemove(m, names, opts, false)
}

// cmdList는 패키지 목록을 보여 줍니다.
func cmdList(m *pkgmgr.Manager, opts options) error {
	if opts.Installed {
		recs := m.DB.List()
		if len(recs) == 0 {
			ui.Info("설치된 패키지가 없습니다.")
			return nil
		}
		for _, r := range recs {
			mark := ""
			if r.Auto {
				mark = " [자동]"
			}
			ui.Out("%s/%s %s%s", r.Name, r.Architecture, r.Version, mark)
		}
		return nil
	}

	if err := m.LoadIndex(); err != nil {
		return err
	}
	for _, name := range m.Index.Names() {
		p := m.Index.Packages[name]
		inst, installed := m.DB.Get(name)

		upgradable := installed && deb.CompareVersions(p.Version, inst.Version) > 0
		if opts.Upgradable && !upgradable {
			continue
		}

		switch {
		case upgradable:
			ui.Out("%s/%s %s [설치됨: %s, 업그레이드 가능]", p.Name, p.Architecture, p.Version, inst.Version)
		case installed:
			ui.Out("%s/%s %s [설치됨]", p.Name, p.Architecture, p.Version)
		default:
			ui.Out("%s/%s %s", p.Name, p.Architecture, p.Version)
		}
	}
	return nil
}

// cmdSearch는 이름과 설명에서 검색합니다.
func cmdSearch(m *pkgmgr.Manager, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("검색어가 필요합니다 (예: rpt search rport)")
	}
	if err := m.LoadIndex(); err != nil {
		return err
	}
	term := strings.ToLower(strings.Join(args, " "))

	found := 0
	for _, name := range m.Index.Names() {
		p := m.Index.Packages[name]
		hay := strings.ToLower(p.Name + " " + p.Description)
		if !strings.Contains(hay, term) {
			continue
		}
		found++
		mark := ""
		if _, ok := m.DB.Get(name); ok {
			mark = " [설치됨]"
		}
		ui.Out("%s/%s %s%s", p.Name, p.Architecture, p.Version, mark)
		if s := p.Summary(); s != "" {
			ui.Out("  %s", s)
		}
	}
	if found == 0 {
		ui.Info("검색 결과가 없습니다: %s", term)
	}
	return nil
}

// cmdShow는 패키지 상세 정보를 보여 줍니다.
func cmdShow(m *pkgmgr.Manager, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("패키지 이름이 필요합니다 (예: rpt show krfs-rport)")
	}
	if err := m.LoadIndex(); err != nil {
		return err
	}
	for i, name := range args {
		p, ok := m.Index.Get(name)
		if !ok {
			return fmt.Errorf("패키지를 찾을 수 없습니다: %s", name)
		}
		if i > 0 {
			ui.Out("")
		}
		ui.Out("패키지: %s", p.Name)
		ui.Out("버전: %s", p.Version)
		ui.Out("아키텍처: %s", p.Architecture)
		if p.Section != "" {
			ui.Out("분류: %s", p.Section)
		}
		if p.Maintainer != "" {
			ui.Out("관리자: %s", p.Maintainer)
		}
		if p.Homepage != "" {
			ui.Out("홈페이지: %s", p.Homepage)
		}
		ui.Out("크기: %s", ui.HumanSize(p.Size))
		if len(p.Depends) > 0 {
			var names []string
			for _, d := range p.Depends {
				names = append(names, d.Raw)
			}
			ui.Out("의존성: %s", strings.Join(names, ", "))
		}
		if inst, ok := m.DB.Get(name); ok {
			ui.Out("설치 상태: %s 설치됨 (%s)", inst.Version, inst.InstalledAt.Format("2006-01-02 15:04"))
			ui.Out("설치 파일: %d개", len(inst.Files))
		} else {
			ui.Out("설치 상태: 설치되지 않음")
		}
		if p.Description != "" {
			ui.Out("설명:")
			ui.Out("%s", ui.Indent(p.Description, "  "))
		}
	}
	return nil
}

// cmdClean은 내려받아 둔 deb 파일을 정리합니다.
func cmdClean(m *pkgmgr.Manager, auto bool) error {
	if err := requireRoot(); err != nil {
		return err
	}
	var (
		res pkgmgr.CleanResult
		err error
	)
	if auto {
		// 저장소 목록이 있어야 어떤 파일이 옛것인지 판단할 수 있다.
		if lerr := m.LoadIndex(); lerr != nil && !errors.Is(lerr, repo.ErrNoIndex) {
			return lerr
		}
		if m.Index == nil {
			return fmt.Errorf("패키지 목록이 없어 옛 파일을 가릴 수 없습니다. 먼저 rpt update 를 실행하십시오")
		}
		res, err = m.AutoClean()
	} else {
		res, err = m.Clean()
	}
	if err != nil {
		return err
	}
	if res.Files == 0 {
		ui.Info("정리할 캐시 파일이 없습니다.")
		return nil
	}
	ui.Info("캐시 파일 %d개를 지웠습니다 (%s 확보).", res.Files, ui.HumanSize(res.Bytes))
	return nil
}

// cmdRelink는 시스템 심링크를 다시 만듭니다.
func cmdRelink(m *pkgmgr.Manager) error {
	if err := requireRoot(); err != nil {
		return err
	}
	results, err := m.Relink()
	if err != nil {
		return err
	}
	if len(results) == 0 {
		ui.Info("설치된 패키지가 없습니다.")
		return nil
	}
	total := 0
	for _, r := range results {
		total += len(r.Created)
		for _, p := range r.Skipped {
			ui.Warn("%s: %s 는 이미 다른 파일이 있어 건너뛰었습니다.", r.Package, p)
		}
	}
	ui.Info("심링크 %d개를 확인했습니다.", total)
	return nil
}
