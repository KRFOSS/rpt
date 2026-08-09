// Package config는 rpt의 실행 설정과 경로 배치를 담당합니다.
//
// rpt는 시놀로지 DSM처럼 apt/dpkg가 없는 환경에서 ROKFOSS 저장소의
// 데비안 패키지를 관리합니다. 시스템 패키지 관리자와 절대 섞이지 않도록
// 모든 상태를 자체 루트 아래에만 기록합니다.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	// DefaultRepoURL은 ROKFOSS 데비안 저장소 주소입니다.
	DefaultRepoURL = "https://pkg.krfoss.org/debian"
	// DefaultDist는 사용할 배포판(Suite)입니다.
	DefaultDist = "stable"
	// DefaultComponent는 사용할 컴포넌트입니다.
	DefaultComponent = "main"
	// DefaultBinDir은 실행 파일 심링크를 걸 시스템 경로입니다.
	DefaultBinDir = "/usr/local/bin"
	// DefaultEtcDir은 설정 디렉터리 심링크를 걸 시스템 경로입니다.
	DefaultEtcDir = "/etc"
	// UserAgent는 저장소 요청에 사용할 식별자입니다.
	UserAgent = "rpt/" + Version + " (ROKFOSS PROJECT)"
	// Version은 rpt 자체 버전입니다.
	Version = "1.0.0"
)

// Config는 한 번의 rpt 실행에 적용되는 설정입니다.
type Config struct {
	// Root는 패키지 파일이 실제로 풀리는 설치 루트입니다.
	// DSM 업데이트 시 시스템 파티션이 재이미징되므로 볼륨 경로를 씁니다.
	Root string
	// RepoURL은 저장소 기본 주소입니다.
	RepoURL string
	// Dist는 배포판 이름입니다.
	Dist string
	// Component는 컴포넌트 이름입니다.
	Component string
	// Arch는 설치 대상 데비안 아키텍처입니다. 이 값과 all 이외의
	// 아키텍처를 가진 패키지는 목록 단계에서 걸러집니다.
	Arch string
	// BinDir은 실행 파일 심링크를 만들 시스템 디렉터리입니다.
	BinDir string
	// EtcDir은 설정 디렉터리 심링크를 만들 시스템 디렉터리입니다.
	EtcDir string
}

// Load는 환경 변수와 실행 환경을 반영한 설정을 만듭니다.
func Load() (*Config, error) {
	root := os.Getenv("RPT_ROOT")
	if root == "" {
		root = detectRoot()
	}
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("설치 루트는 절대 경로여야 합니다: %s", root)
	}
	if filepath.Clean(root) == "/" {
		return nil, fmt.Errorf("설치 루트를 / 로 지정할 수 없습니다. rpt는 시스템 경로에 직접 설치하지 않습니다")
	}

	repoURL := os.Getenv("RPT_REPO")
	if repoURL == "" {
		repoURL = DefaultRepoURL
	}
	binDir := os.Getenv("RPT_BINDIR")
	if binDir == "" {
		binDir = DefaultBinDir
	}
	etcDir := os.Getenv("RPT_ETCDIR")
	if etcDir == "" {
		etcDir = DefaultEtcDir
	}

	return &Config{
		Root:      filepath.Clean(root),
		RepoURL:   repoURL,
		Dist:      DefaultDist,
		Component: DefaultComponent,
		Arch:      DebArch(),
		BinDir:    binDir,
		EtcDir:    etcDir,
	}, nil
}

// detectRoot는 설치 루트를 자동으로 고릅니다.
//
// 시놀로지에서는 /volumeN 이 존재하므로 첫 번째 볼륨 아래를 쓰고,
// 그 외 환경에서는 /opt 아래로 떨어집니다. 시스템 파티션을 피하는 것이
// 목적이며, DSM 업데이트로 설치물이 날아가는 것을 막습니다.
func detectRoot() string {
	for i := 1; i <= 8; i++ {
		vol := fmt.Sprintf("/volume%d", i)
		if fi, err := os.Stat(vol); err == nil && fi.IsDir() {
			return filepath.Join(vol, "@rokfoss")
		}
	}
	return "/opt/rokfoss"
}

// DebArch는 현재 실행 중인 아키텍처를 데비안 표기로 변환합니다.
func DebArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "amd64"
	case "arm64":
		return "arm64"
	case "386":
		return "i386"
	case "arm":
		return "armhf"
	default:
		return runtime.GOARCH
	}
}

// StateDir은 rpt의 상태 파일이 놓이는 디렉터리입니다.
func (c *Config) StateDir() string { return filepath.Join(c.Root, "var/lib/rpt") }

// StatusFile은 설치 목록 데이터베이스 경로입니다.
func (c *Config) StatusFile() string { return filepath.Join(c.StateDir(), "status.json") }

// ListsDir은 저장소 메타데이터 캐시 디렉터리입니다.
func (c *Config) ListsDir() string { return filepath.Join(c.StateDir(), "lists") }

// CacheDir은 내려받은 deb 파일을 두는 디렉터리입니다.
func (c *Config) CacheDir() string { return filepath.Join(c.Root, "var/cache/rpt/archives") }
