package pkgmgr

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/krfoss/rpt/internal/state"
)

// RemovePlan은 제거 작업 계획입니다.
type RemovePlan struct {
	Items []*state.Installed
	// Purge가 참이면 설정 파일까지 지웁니다.
	Purge bool
	// Blocked는 다른 패키지가 의존하고 있어 지울 수 없는 항목입니다.
	// 키가 대상 패키지, 값이 그것을 필요로 하는 패키지들입니다.
	Blocked map[string][]string
	// NotInstalled는 애초에 설치되어 있지 않은 이름입니다.
	NotInstalled []string
}

// Empty는 지울 것이 없는 계획인지 알려줍니다.
func (p *RemovePlan) Empty() bool { return len(p.Items) == 0 }

// PlanRemove는 제거 계획을 세웁니다.
func (m *Manager) PlanRemove(names []string, purge bool) *RemovePlan {
	plan := &RemovePlan{Purge: purge, Blocked: map[string][]string{}}

	targets := map[string]bool{}
	for _, n := range names {
		if _, ok := m.DB.Get(n); !ok {
			plan.NotInstalled = append(plan.NotInstalled, n)
			continue
		}
		targets[n] = true
	}

	depsOf := func(name string) []string {
		if rec, ok := m.DB.Get(name); ok {
			return rec.Depends
		}
		return nil
	}

	for name := range targets {
		// 함께 지우는 패키지가 의존하는 것은 막을 이유가 없다.
		var blockers []string
		for _, r := range m.DB.RequiredBy(name, depsOf) {
			if !targets[r] {
				blockers = append(blockers, r)
			}
		}
		if len(blockers) > 0 {
			sort.Strings(blockers)
			plan.Blocked[name] = blockers
			continue
		}
		rec, _ := m.DB.Get(name)
		plan.Items = append(plan.Items, rec)
	}

	sort.Slice(plan.Items, func(i, j int) bool { return plan.Items[i].Name < plan.Items[j].Name })
	sort.Strings(plan.NotInstalled)
	return plan
}

// RemoveResult는 패키지 하나를 지운 결과입니다.
type RemoveResult struct {
	Name string
	// RemovedFiles는 실제로 지운 파일 수입니다.
	RemovedFiles int
	// KeptConffiles는 남겨 둔 설정 파일입니다. purge가 아니면 남습니다.
	KeptConffiles []string
	// RemovedLinks는 걷어낸 시스템 심링크입니다.
	RemovedLinks []string
}

// ExecuteRemove는 제거 계획을 수행합니다.
func (m *Manager) ExecuteRemove(plan *RemovePlan, onStep func(i, total int, rec *state.Installed)) ([]RemoveResult, error) {
	var out []RemoveResult
	for i, rec := range plan.Items {
		if onStep != nil {
			onStep(i+1, len(plan.Items), rec)
		}
		res := m.removeOne(rec, plan.Purge)
		out = append(out, res)
		m.DB.Delete(rec.Name)
	}
	if err := m.DB.Save(); err != nil {
		return out, err
	}
	return out, nil
}

// removeOne은 패키지 하나의 파일과 링크를 걷어냅니다.
func (m *Manager) removeOne(rec *state.Installed, purge bool) RemoveResult {
	res := RemoveResult{Name: rec.Name}
	res.RemovedLinks = m.removeLinks(rec)

	conf := map[string]bool{}
	for _, c := range rec.Conffiles {
		conf[c] = true
	}

	for _, rel := range rec.Files {
		if conf[rel] && !purge {
			res.KeptConffiles = append(res.KeptConffiles, rel)
			continue
		}
		p, err := m.resolve(rel)
		if err != nil {
			continue
		}
		if err := os.Remove(p); err == nil {
			res.RemovedFiles++
		}
	}

	m.pruneDirs(rec.Dirs)
	return res
}

// pruneDirs는 패키지가 만들었던 디렉터리 중 빈 것을 깊은 곳부터 정리합니다.
//
// os.Remove는 비어 있는 디렉터리만 지우므로, 다른 패키지가 아직 쓰고 있는
// 디렉터리는 자연히 남습니다.
func (m *Manager) pruneDirs(dirs []string) {
	sorted := append([]string(nil), dirs...)
	sort.Slice(sorted, func(i, j int) bool {
		di, dj := countSep(sorted[i]), countSep(sorted[j])
		if di != dj {
			return di > dj
		}
		return sorted[i] > sorted[j]
	})
	for _, rel := range sorted {
		p, err := m.resolve(rel)
		if err != nil || p == m.Cfg.Root {
			continue
		}
		_ = os.Remove(p)
	}
}

// countSep는 경로의 깊이를 셉니다.
func countSep(p string) int {
	n := 0
	for _, c := range p {
		if c == '/' {
			n++
		}
	}
	return n
}

// AutoRemovable은 더 이상 필요 없는 자동 설치 패키지를 찾습니다.
func (m *Manager) AutoRemovable() []string {
	depsOf := func(name string) []string {
		if rec, ok := m.DB.Get(name); ok {
			return rec.Depends
		}
		return nil
	}
	var out []string
	for _, rec := range m.DB.List() {
		if !rec.Auto {
			continue
		}
		if len(m.DB.RequiredBy(rec.Name, depsOf)) == 0 {
			out = append(out, rec.Name)
		}
	}
	return out
}

// CleanResult는 캐시 정리 결과입니다.
type CleanResult struct {
	Files int
	Bytes int64
}

// Clean은 내려받아 둔 deb 파일을 모두 지웁니다.
func (m *Manager) Clean() (CleanResult, error) {
	return m.cleanCache(func(name string) bool { return true })
}

// AutoClean은 지금 저장소 목록에 없는 옛 deb 파일만 지웁니다.
func (m *Manager) AutoClean() (CleanResult, error) {
	keep := map[string]bool{}
	if m.Index != nil {
		for _, p := range m.Index.Packages {
			keep[filepath.Base(p.Filename)] = true
		}
	}
	return m.cleanCache(func(name string) bool { return !keep[name] })
}

// cleanCache는 조건에 맞는 캐시 파일을 지웁니다.
func (m *Manager) cleanCache(shouldDelete func(name string) bool) (CleanResult, error) {
	var res CleanResult
	entries, err := os.ReadDir(m.Cfg.CacheDir())
	if err != nil {
		if os.IsNotExist(err) {
			return res, nil
		}
		return res, err
	}
	for _, e := range entries {
		if e.IsDir() || !shouldDelete(e.Name()) {
			continue
		}
		p := filepath.Join(m.Cfg.CacheDir(), e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}
		if err := os.Remove(p); err != nil {
			return res, fmt.Errorf("캐시 파일을 지우지 못했습니다 (%s): %w", p, err)
		}
		res.Files++
		res.Bytes += info.Size()
	}
	return res, nil
}
