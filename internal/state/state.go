// Package state는 rpt가 설치한 패키지 목록을 보관합니다.
//
// dpkg의 /var/lib/dpkg 는 읽지도 쓰지도 않습니다. rpt가 만든 것만
// rpt가 지울 수 있어야 시스템 패키지 관리자와 절대 충돌하지 않습니다.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// schemaVersion은 상태 파일 형식 버전입니다.
const schemaVersion = 1

// Link는 rpt가 시스템 경로에 만든 심링크 하나입니다.
// 여기 기록된 것만 제거 대상이 되므로, 남이 만든 링크는 절대 지우지 않습니다.
type Link struct {
	// Path는 시스템 경로(예: /usr/local/bin/dsync)입니다.
	Path string `json:"path"`
	// Target은 링크가 가리키는 설치 루트 안의 실제 경로입니다.
	Target string `json:"target"`
}

// Installed는 설치된 패키지 하나의 기록입니다.
type Installed struct {
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Architecture string    `json:"architecture"`
	Section      string    `json:"section,omitempty"`
	Description  string    `json:"description,omitempty"`
	InstalledAt  time.Time `json:"installed_at"`
	// Files는 설치 루트 기준 상대 경로입니다.
	Files []string `json:"files"`
	// Dirs는 이 패키지가 만든 디렉터리입니다. 제거할 때 비어 있으면 정리합니다.
	Dirs []string `json:"dirs,omitempty"`
	// Links는 rpt가 만든 시스템 심링크입니다.
	Links []Link `json:"links,omitempty"`
	// Conffiles는 설정 파일 경로입니다. remove는 남기고 purge만 지웁니다.
	Conffiles []string `json:"conffiles,omitempty"`
	// Depends는 이 패키지가 의존하는 저장소 안의 패키지 이름입니다.
	// 목록 캐시가 없어도 역의존성을 따질 수 있도록 함께 저장합니다.
	Depends []string `json:"depends,omitempty"`
	// Auto는 의존성 때문에 딸려 설치되었는지를 뜻합니다.
	Auto bool `json:"auto,omitempty"`
	// SkippedScripts는 실행하지 않은 관리자 스크립트 이름입니다.
	SkippedScripts []string `json:"skipped_scripts,omitempty"`
}

// DB는 설치 상태 전체입니다.
type DB struct {
	SchemaVersion int                   `json:"schema_version"`
	Packages      map[string]*Installed `json:"packages"`

	path string
}

// Load는 상태 파일을 읽습니다. 파일이 없으면 빈 상태로 시작합니다.
func Load(path string) (*DB, error) {
	db := &DB{
		SchemaVersion: schemaVersion,
		Packages:      map[string]*Installed{},
		path:          path,
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return db, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(b, db); err != nil {
		return nil, fmt.Errorf("설치 상태 파일이 손상되었습니다 (%s): %w", path, err)
	}
	if db.Packages == nil {
		db.Packages = map[string]*Installed{}
	}
	db.path = path
	return db, nil
}

// Save는 상태를 원자적으로 기록합니다.
func (db *DB) Save() error {
	if err := os.MkdirAll(filepath.Dir(db.path), 0o755); err != nil {
		return err
	}
	db.SchemaVersion = schemaVersion
	b, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	tmp := db.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, db.path)
}

// Get은 설치 기록을 찾습니다.
func (db *DB) Get(name string) (*Installed, bool) {
	p, ok := db.Packages[name]
	return p, ok
}

// Put은 설치 기록을 넣거나 갱신합니다.
func (db *DB) Put(p *Installed) { db.Packages[p.Name] = p }

// Delete는 설치 기록을 지웁니다.
func (db *DB) Delete(name string) { delete(db.Packages, name) }

// Names는 설치된 패키지 이름을 사전순으로 돌려줍니다.
func (db *DB) Names() []string {
	names := make([]string, 0, len(db.Packages))
	for n := range db.Packages {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// List는 설치 기록을 이름순으로 돌려줍니다.
func (db *DB) List() []*Installed {
	out := make([]*Installed, 0, len(db.Packages))
	for _, n := range db.Names() {
		out = append(out, db.Packages[n])
	}
	return out
}

// RequiredBy는 주어진 패키지를 의존하는 설치된 패키지 이름을 찾습니다.
// deps는 패키지 이름에서 그 패키지의 의존 목록을 얻는 함수입니다.
func (db *DB) RequiredBy(name string, deps func(string) []string) []string {
	var out []string
	for _, other := range db.Names() {
		if other == name {
			continue
		}
		for _, d := range deps(other) {
			if d == name {
				out = append(out, other)
				break
			}
		}
	}
	return out
}
