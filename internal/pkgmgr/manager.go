// Package pkgmgr는 설치, 제거, 업그레이드 같은 실제 작업을 수행합니다.
package pkgmgr

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/krfoss/rpt/internal/config"
	"github.com/krfoss/rpt/internal/deb"
	"github.com/krfoss/rpt/internal/repo"
	"github.com/krfoss/rpt/internal/state"
)

// Manager는 한 번의 작업에 필요한 설정, 저장소, 상태를 묶습니다.
type Manager struct {
	Cfg    *config.Config
	Client *repo.Client
	DB     *state.DB
	Index  *repo.Index
}

// New는 상태 파일을 읽어 관리자를 만듭니다.
// 저장소 목록 캐시는 아직 읽지 않습니다.
func New(cfg *config.Config) (*Manager, error) {
	db, err := state.Load(cfg.StatusFile())
	if err != nil {
		return nil, err
	}
	return &Manager{Cfg: cfg, Client: repo.NewClient(cfg), DB: db}, nil
}

// LoadIndex는 저장소 목록 캐시를 읽어 둡니다.
func (m *Manager) LoadIndex() error {
	ix, err := m.Client.Load()
	if err != nil {
		return err
	}
	m.Index = ix
	return nil
}

// Action은 계획된 작업의 종류입니다.
type Action int

const (
	// ActionInstall은 새로 설치하는 경우입니다.
	ActionInstall Action = iota
	// ActionUpgrade는 더 높은 버전으로 올리는 경우입니다.
	ActionUpgrade
	// ActionReinstall은 같은 버전을 다시 까는 경우입니다.
	ActionReinstall
)

// String은 작업 종류의 한국어 이름입니다.
func (a Action) String() string {
	switch a {
	case ActionUpgrade:
		return "업그레이드"
	case ActionReinstall:
		return "재설치"
	default:
		return "설치"
	}
}

// PlanItem은 처리할 패키지 하나입니다.
type PlanItem struct {
	Pkg *repo.Package
	// Action은 이 패키지에 수행할 작업입니다.
	Action Action
	// Auto는 사용자가 직접 지정하지 않고 의존성으로 딸려온 경우 참입니다.
	Auto bool
	// Current는 이미 설치된 버전입니다. 새 설치면 빈 문자열입니다.
	Current string
}

// Plan은 설치 작업 계획입니다.
type Plan struct {
	Items []PlanItem
	// MissingDeps는 저장소에 없는 의존성입니다. 시스템이 이미 제공하는
	// 경우가 대부분이라 경고만 하고 진행합니다.
	MissingDeps []string
	// AlreadyCurrent는 이미 최신이라 건드릴 필요가 없는 패키지입니다.
	AlreadyCurrent []string
	// DownloadSize는 내려받아야 할 총 바이트입니다.
	DownloadSize int64
}

// Empty는 할 일이 없는 계획인지 알려줍니다.
func (p *Plan) Empty() bool { return len(p.Items) == 0 }

// PlanInstall은 지정한 패키지와 그 의존성의 설치 계획을 세웁니다.
//
// reinstall이 참이면 같은 버전이 이미 있어도 다시 설치합니다.
func (m *Manager) PlanInstall(names []string, reinstall bool) (*Plan, error) {
	if m.Index == nil {
		return nil, repo.ErrNoIndex
	}
	plan := &Plan{}
	seen := map[string]bool{}
	missing := map[string]bool{}

	// 사용자가 직접 지정한 패키지부터 훑고, 의존성은 뒤에 이어 붙인다.
	type queued struct {
		name string
		auto bool
	}
	queue := make([]queued, 0, len(names))
	for _, n := range names {
		queue = append(queue, queued{name: n})
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if seen[cur.name] {
			continue
		}
		seen[cur.name] = true

		pkg, ok := m.Index.Get(cur.name)
		if !ok {
			if cur.auto {
				// 저장소 밖 의존성은 시스템이 제공한다고 보고 경고만 한다.
				missing[cur.name] = true
				continue
			}
			return nil, fmt.Errorf("패키지를 찾을 수 없습니다: %s (rpt update 후 rpt search 로 확인해 보십시오)", cur.name)
		}

		action := ActionInstall
		current := ""
		if inst, ok := m.DB.Get(cur.name); ok {
			current = inst.Version
			switch c := deb.CompareVersions(pkg.Version, inst.Version); {
			case c > 0:
				action = ActionUpgrade
			case c == 0:
				if !reinstall {
					if !cur.auto {
						plan.AlreadyCurrent = append(plan.AlreadyCurrent, cur.name)
					}
					// 이미 최신이어도 의존성은 계속 따라간다.
					for _, d := range pkg.Depends {
						queue = append(queue, queued{name: d.Name, auto: true})
					}
					continue
				}
				action = ActionReinstall
			default:
				// 저장소가 더 낮은 버전을 들고 있으면 손대지 않는다.
				if !cur.auto {
					plan.AlreadyCurrent = append(plan.AlreadyCurrent, cur.name)
				}
				continue
			}
		}

		plan.Items = append(plan.Items, PlanItem{
			Pkg: pkg, Action: action, Auto: cur.auto, Current: current,
		})
		plan.DownloadSize += pkg.Size

		for _, d := range pkg.Depends {
			queue = append(queue, queued{name: d.Name, auto: true})
		}
	}

	for d := range missing {
		plan.MissingDeps = append(plan.MissingDeps, d)
	}
	sort.Strings(plan.MissingDeps)
	sort.Strings(plan.AlreadyCurrent)
	return plan, nil
}

// PlanUpgrade는 설치된 패키지 중 새 버전이 있는 것들의 계획을 세웁니다.
func (m *Manager) PlanUpgrade(names []string) (*Plan, error) {
	if m.Index == nil {
		return nil, repo.ErrNoIndex
	}
	if len(names) == 0 {
		names = m.DB.Names()
		if len(names) == 0 {
			return &Plan{}, nil
		}
	}
	var targets []string
	for _, n := range names {
		if _, ok := m.DB.Get(n); !ok {
			return nil, fmt.Errorf("설치되어 있지 않은 패키지입니다: %s", n)
		}
		targets = append(targets, n)
	}
	plan, err := m.PlanInstall(targets, false)
	if err != nil {
		return nil, err
	}
	// upgrade는 이미 최신인 패키지를 따로 보고하지 않는다.
	plan.AlreadyCurrent = nil
	return plan, nil
}

// InstallResult는 패키지 하나를 설치한 결과입니다.
type InstallResult struct {
	Name    string
	Version string
	Action  Action
	// Cached는 캐시에 있던 파일을 그대로 썼는지 여부입니다.
	Cached bool
	// Links는 새로 만든 시스템 심링크입니다.
	Links []state.Link
	// SkippedLinks는 이미 다른 것이 차지하고 있어 만들지 않은 링크입니다.
	SkippedLinks []string
	// KeptConffiles는 덮어쓰지 않고 남겨 둔 설정 파일입니다.
	KeptConffiles []string
	// SkippedScripts는 실행하지 않은 관리자 스크립트입니다.
	SkippedScripts []string
}

// Execute는 계획을 실제로 수행합니다.
// 진행 상황은 onStep으로 알려 줍니다.
func (m *Manager) Execute(plan *Plan, onStep func(i, total int, item PlanItem)) ([]InstallResult, error) {
	var results []InstallResult
	for i, item := range plan.Items {
		if onStep != nil {
			onStep(i+1, len(plan.Items), item)
		}
		res, err := m.installOne(item)
		if err != nil {
			// 여기까지 성공한 것은 상태에 남기고 멈춘다.
			if serr := m.DB.Save(); serr != nil {
				return results, fmt.Errorf("%w (설치 상태 저장도 실패했습니다: %v)", err, serr)
			}
			return results, err
		}
		results = append(results, *res)
	}
	if err := m.DB.Save(); err != nil {
		return results, err
	}
	return results, nil
}

// installOne은 패키지 하나를 내려받아 설치하고 상태에 기록합니다.
func (m *Manager) installOne(item PlanItem) (*InstallResult, error) {
	path, cached, err := m.Client.Download(item.Pkg)
	if err != nil {
		return nil, err
	}

	archive, err := deb.Open(path)
	if err != nil {
		return nil, err
	}
	if archive.Name() != item.Pkg.Name {
		return nil, fmt.Errorf("내려받은 파일의 패키지 이름이 다릅니다 (기대 %s, 실제 %s)",
			item.Pkg.Name, archive.Name())
	}

	// 업그레이드나 재설치라면 옛 파일 중 새 목록에 없는 것을 먼저 치운다.
	var old *state.Installed
	if prev, ok := m.DB.Get(item.Pkg.Name); ok {
		old = prev
	}

	if err := os.MkdirAll(m.Cfg.Root, 0o755); err != nil {
		return nil, err
	}
	ex, err := archive.Extract(m.Cfg.Root, true)
	if err != nil {
		return nil, err
	}

	rec := &state.Installed{
		Name:           item.Pkg.Name,
		Version:        item.Pkg.Version,
		Architecture:   item.Pkg.Architecture,
		Section:        item.Pkg.Section,
		Description:    item.Pkg.Description,
		InstalledAt:    nowFunc(),
		Files:          ex.Files,
		Dirs:           ex.Dirs,
		Conffiles:      conffileList(archive),
		Depends:        m.repoDepNames(item.Pkg),
		Auto:           item.Auto,
		SkippedScripts: archive.Scripts,
	}
	// 사용자가 직접 설치한 적이 있으면 자동 표시를 유지하지 않는다.
	if old != nil && !old.Auto {
		rec.Auto = false
	}

	if old != nil {
		m.removeStaleFiles(old, ex.Files)
	}

	links, skipped := m.createLinks(rec)
	rec.Links = links

	m.DB.Put(rec)

	return &InstallResult{
		Name:           rec.Name,
		Version:        rec.Version,
		Action:         item.Action,
		Cached:         cached,
		Links:          links,
		SkippedLinks:   skipped,
		KeptConffiles:  ex.KeptConffiles,
		SkippedScripts: archive.Scripts,
	}, nil
}

// removeStaleFiles는 이전 버전에만 있던 파일을 지웁니다.
func (m *Manager) removeStaleFiles(old *state.Installed, newFiles []string) {
	keep := map[string]bool{}
	for _, f := range newFiles {
		keep[f] = true
	}
	conf := map[string]bool{}
	for _, c := range old.Conffiles {
		conf[c] = true
	}
	for _, f := range old.Files {
		if keep[f] || conf[f] {
			continue
		}
		if p, err := m.resolve(f); err == nil {
			_ = os.Remove(p)
		}
	}
}

// repoDepNames는 저장소 안에 실제로 존재하는 의존성 이름만 추립니다.
func (m *Manager) repoDepNames(p *repo.Package) []string {
	var out []string
	for _, d := range p.Depends {
		if m.Index != nil {
			if _, ok := m.Index.Get(d.Name); !ok {
				continue
			}
		}
		out = append(out, d.Name)
	}
	return out
}

// conffileList는 아카이브의 설정 파일 목록을 정렬해 돌려줍니다.
func conffileList(a *deb.Archive) []string {
	out := make([]string, 0, len(a.Conffiles))
	for c := range a.Conffiles {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// resolve는 설치 루트 기준 상대 경로를 절대 경로로 바꿉니다.
// 루트를 벗어나면 오류를 냅니다.
func (m *Manager) resolve(rel string) (string, error) {
	full := filepath.Join(m.Cfg.Root, filepath.FromSlash(rel))
	r, err := filepath.Rel(m.Cfg.Root, full)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("설치 루트를 벗어나는 경로입니다: %s", rel)
	}
	return full, nil
}

// nowFunc는 시험에서 바꿔 끼울 수 있도록 분리해 둔 현재 시각 함수입니다.
var nowFunc = timeNow
