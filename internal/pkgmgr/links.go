package pkgmgr

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/krfoss/rpt/internal/state"
)

// timeNow는 현재 시각을 돌려줍니다.
func timeNow() time.Time { return time.Now() }

// binDirs는 실행 파일이 들어가는 패키지 내부 경로입니다.
var binDirs = []string{"usr/bin/", "usr/sbin/", "bin/", "sbin/"}

// desiredLinks는 설치 기록으로부터 만들어야 할 시스템 심링크를 계산합니다.
//
// 실행 파일은 PATH에 잡히도록 /usr/local/bin 에 걸고, 설정 디렉터리는
// 데비안용으로 빌드된 바이너리가 /etc/<이름> 을 그대로 찾기 때문에
// 최상위 디렉터리 단위로 걸어 줍니다.
func (m *Manager) desiredLinks(rec *state.Installed) []state.Link {
	var links []state.Link
	seen := map[string]bool{}

	add := func(sysPath, target string) {
		if seen[sysPath] {
			return
		}
		seen[sysPath] = true
		links = append(links, state.Link{Path: sysPath, Target: target})
	}

	for _, rel := range rec.Files {
		for _, d := range binDirs {
			if strings.HasPrefix(rel, d) && !strings.Contains(rel[len(d):], "/") {
				target, err := m.resolve(rel)
				if err != nil {
					continue
				}
				add(filepath.Join(m.Cfg.BinDir, filepath.Base(rel)), target)
			}
		}
		if strings.HasPrefix(rel, "etc/") {
			rest := strings.TrimPrefix(rel, "etc/")
			top := rest
			if i := strings.IndexByte(rest, '/'); i >= 0 {
				top = rest[:i]
			}
			if top == "" {
				continue
			}
			target, err := m.resolve("etc/" + top)
			if err != nil {
				continue
			}
			add(filepath.Join(m.Cfg.EtcDir, top), target)
		}
	}

	sort.Slice(links, func(i, j int) bool { return links[i].Path < links[j].Path })
	return links
}

// createLinks는 심링크를 만들고, 실제로 만든 것과 건너뛴 것을 돌려줍니다.
//
// 목적지에 이미 다른 것이 있으면 절대 건드리지 않습니다. 남이 만든 파일을
// 지우는 일이 없어야 시스템과 충돌하지 않습니다.
func (m *Manager) createLinks(rec *state.Installed) (created []state.Link, skipped []string) {
	for _, l := range m.desiredLinks(rec) {
		if err := os.MkdirAll(filepath.Dir(l.Path), 0o755); err != nil {
			skipped = append(skipped, l.Path)
			continue
		}
		switch fi, err := os.Lstat(l.Path); {
		case err != nil && os.IsNotExist(err):
			if err := os.Symlink(l.Target, l.Path); err != nil {
				skipped = append(skipped, l.Path)
				continue
			}
			created = append(created, l)

		case err != nil:
			skipped = append(skipped, l.Path)

		case fi.Mode()&os.ModeSymlink != 0:
			dest, rerr := os.Readlink(l.Path)
			if rerr == nil && dest == l.Target {
				// 이미 우리가 건 링크다.
				created = append(created, l)
				continue
			}
			if rerr == nil && m.ownsPath(dest) {
				// 이전 버전이 남긴 우리 링크이므로 새 대상으로 갱신한다.
				if os.Remove(l.Path) == nil && os.Symlink(l.Target, l.Path) == nil {
					created = append(created, l)
					continue
				}
			}
			skipped = append(skipped, l.Path)

		default:
			// 일반 파일이나 디렉터리가 이미 있으면 그대로 둔다.
			skipped = append(skipped, l.Path)
		}
	}
	return created, skipped
}

// removeLinks는 rpt가 만든 심링크만 걷어냅니다.
// 기록에 있더라도 지금 우리 설치 루트를 가리키는 심링크가 아니면 남깁니다.
func (m *Manager) removeLinks(rec *state.Installed) []string {
	var removed []string
	for _, l := range rec.Links {
		fi, err := os.Lstat(l.Path)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			continue
		}
		dest, err := os.Readlink(l.Path)
		if err != nil || !m.ownsPath(dest) {
			continue
		}
		if os.Remove(l.Path) == nil {
			removed = append(removed, l.Path)
		}
	}
	return removed
}

// ownsPath는 주어진 경로가 rpt 설치 루트 안에 있는지 판단합니다.
func (m *Manager) ownsPath(p string) bool {
	if !filepath.IsAbs(p) {
		return false
	}
	rel, err := filepath.Rel(m.Cfg.Root, filepath.Clean(p))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// RelinkResult는 심링크 복구 결과입니다.
type RelinkResult struct {
	Package string
	Created []state.Link
	Skipped []string
}

// Relink는 설치된 모든 패키지의 시스템 심링크를 다시 만듭니다.
//
// 설치 루트와 달리 심링크는 시스템 경로에 있어 밖에서 지워질 수 있습니다.
// 특히 DSM 업데이트는 시스템 파티션을 다시 씌우므로 /usr/local/bin 의
// 링크가 통째로 사라집니다. 실제 파일은 설치 루트에 남아 있으니 링크만
// 되살리면 그대로 다시 쓸 수 있습니다.
func (m *Manager) Relink() ([]RelinkResult, error) {
	var out []RelinkResult
	for _, rec := range m.DB.List() {
		created, skipped := m.createLinks(rec)
		rec.Links = created
		out = append(out, RelinkResult{Package: rec.Name, Created: created, Skipped: skipped})
	}
	if err := m.DB.Save(); err != nil {
		return out, err
	}
	return out, nil
}
