// Package repo는 ROKFOSS 데비안 저장소의 메타데이터를 가져오고 검증합니다.
//
// 신뢰의 사슬은 세 단계입니다. InRelease를 내장 GPG 키로 검증하고,
// 그 안에 적힌 SHA256으로 Packages를 검증하고, Packages에 적힌 SHA256으로
// 실제 deb 파일을 검증합니다.
package repo

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/krfoss/rpt/internal/config"
	"github.com/krfoss/rpt/internal/deb"
)

// Package는 저장소 목록에 실린 패키지 하나입니다.
type Package struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Architecture string            `json:"architecture"`
	Filename     string            `json:"filename"`
	Size         int64             `json:"size"`
	SHA256       string            `json:"sha256"`
	Section      string            `json:"section,omitempty"`
	Homepage     string            `json:"homepage,omitempty"`
	Maintainer   string            `json:"maintainer,omitempty"`
	Description  string            `json:"description,omitempty"`
	Depends      []deb.Dependency  `json:"depends,omitempty"`
	Extra        map[string]string `json:"-"`
}

// Summary는 목록 출력에 쓰는 한 줄 설명입니다.
func (p *Package) Summary() string { return deb.ShortDescription(p.Description) }

// Index는 검증을 마친 저장소 패키지 목록입니다.
type Index struct {
	Origin    string              `json:"origin"`
	Label     string              `json:"label"`
	Suite     string              `json:"suite"`
	Codename  string              `json:"codename"`
	Component string              `json:"component"`
	Arch      string              `json:"arch"`
	RepoURL   string              `json:"repo_url"`
	FetchedAt time.Time           `json:"fetched_at"`
	Signer    *SignerInfo         `json:"signer,omitempty"`
	Packages  map[string]*Package `json:"packages"`
	// OtherArches는 저장소에 있지만 이 기기에서 쓰지 않는 아키텍처입니다.
	OtherArches []string `json:"other_arches,omitempty"`
}

// Get은 이름으로 패키지를 찾습니다.
func (ix *Index) Get(name string) (*Package, bool) {
	p, ok := ix.Packages[name]
	return p, ok
}

// Names는 패키지 이름을 사전순으로 돌려줍니다.
func (ix *Index) Names() []string {
	names := make([]string, 0, len(ix.Packages))
	for n := range ix.Packages {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Client는 저장소에 접근하는 HTTP 클라이언트입니다.
type Client struct {
	cfg  *config.Config
	http *http.Client

	// OnRetry는 세대 불일치로 목록을 다시 받기 직전에 호출됩니다.
	// 기다리는 동안 멈춘 것처럼 보이지 않도록 알리는 용도이며,
	// 설정하지 않아도 동작에는 영향이 없습니다.
	OnRetry func(attempt, total int, wait time.Duration)

	// OnFetch는 메타데이터 파일을 하나 받을 때마다 호출됩니다.
	// label은 "stable InRelease" 처럼 저장소 안에서의 위치를 가리킵니다.
	OnFetch func(label string, bytes int64)
}

// reportFetch는 받은 파일을 알립니다.
func (c *Client) reportFetch(label string, bytes int64) {
	if c.OnFetch != nil {
		c.OnFetch(label, bytes)
	}
}

// NewClient는 저장소 클라이언트를 만듭니다.
func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

// indexCacheFile은 검증을 마친 목록을 저장할 캐시 경로입니다.
func (c *Client) indexCacheFile() string {
	return filepath.Join(c.cfg.ListsDir(), "index.json")
}

// ErrMetadataSkew는 Release와 패키지 목록의 세대가 서로 어긋났다는 뜻입니다.
//
// 저장소가 색인을 다시 만드는 도중에 요청이 걸리면, 새로 쓰인 Release와
// 아직 옛것인 Packages를 함께 받게 되어 해시가 맞지 않습니다. 파일이
// 깨진 것이 아니라 잠깐의 과도기이므로 다시 받으면 대개 해결됩니다.
var ErrMetadataSkew = errors.New("저장소 메타데이터의 세대가 서로 맞지 않습니다")

// metadataAttempts는 세대 불일치를 만났을 때 목록 받기를 시도하는 총 횟수입니다.
const metadataAttempts = 3

// metadataRetryWait는 재시도 사이에 기다리는 시간입니다.
// 색인 생성은 보통 순식간에 끝나므로 짧게 잡습니다.
var metadataRetryWait = 2 * time.Second

// Update는 저장소에서 목록을 새로 받아 검증하고 캐시에 저장합니다.
//
// 세대 불일치는 저장소 쪽의 일시적인 상태이므로 몇 번 다시 시도합니다.
// 그 밖의 오류는 곧바로 돌려줍니다.
func (c *Client) Update() (*Index, error) {
	var lastErr error
	for attempt := 1; attempt <= metadataAttempts; attempt++ {
		ix, err := c.updateOnce()
		if err == nil {
			return ix, nil
		}
		if !errors.Is(err, ErrMetadataSkew) {
			return nil, err
		}
		lastErr = err
		if attempt < metadataAttempts {
			if c.OnRetry != nil {
				c.OnRetry(attempt, metadataAttempts, metadataRetryWait)
			}
			time.Sleep(metadataRetryWait)
		}
	}
	return nil, lastErr
}

// updateOnce는 목록 받기와 검증을 한 번 수행합니다.
func (c *Client) updateOnce() (*Index, error) {
	inRelease, err := c.fetch(c.url("dists", c.cfg.Dist, "InRelease"))
	if err != nil {
		return nil, fmt.Errorf("InRelease를 받지 못했습니다: %w", err)
	}
	c.reportFetch(c.cfg.Dist+" InRelease", int64(len(inRelease)))

	body, signer, err := verifyInRelease(inRelease)
	if err != nil {
		return nil, err
	}

	release, err := deb.ParseParagraphs(bytes.NewReader(body))
	if err != nil || len(release) == 0 {
		return nil, fmt.Errorf("Release 내용을 해석하지 못했습니다")
	}
	rel := release[0]

	hashes := parseHashBlock(rel.Get("SHA256"))
	if len(hashes) == 0 {
		return nil, fmt.Errorf("Release에 SHA256 목록이 없습니다")
	}

	// 이 기기의 아키텍처에 해당하는 목록만 내려받는다.
	// 다른 아키텍처(arm64 등)는 아예 요청하지 않으므로 자연히 걸러진다.
	relPathGz := path.Join(c.cfg.Component, "binary-"+c.cfg.Arch, "Packages.gz")
	relPathRaw := path.Join(c.cfg.Component, "binary-"+c.cfg.Arch, "Packages")

	target, compressed := relPathGz, true
	if _, ok := hashes[relPathGz]; !ok {
		if _, ok := hashes[relPathRaw]; !ok {
			return nil, fmt.Errorf("저장소에 %s 아키텍처 패키지 목록이 없습니다", c.cfg.Arch)
		}
		target, compressed = relPathRaw, false
	}

	raw, err := c.fetch(c.url("dists", c.cfg.Dist, target))
	if err != nil {
		return nil, fmt.Errorf("%s를 받지 못했습니다: %w", target, err)
	}
	c.reportFetch(fmt.Sprintf("%s/%s %s Packages", c.cfg.Dist, c.cfg.Component, c.cfg.Arch), int64(len(raw)))
	if err := verifySHA256(raw, hashes[target].hash); err != nil {
		// Release는 서명으로 확인했으므로 어긋난 쪽은 목록이다.
		// 저장소가 색인을 다시 만드는 중일 때 흔히 일어난다.
		return nil, fmt.Errorf("%w (%s: %v)", ErrMetadataSkew, target, err)
	}

	if compressed {
		zr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("%s 압축을 풀지 못했습니다: %w", target, err)
		}
		defer zr.Close()
		if raw, err = io.ReadAll(zr); err != nil {
			return nil, fmt.Errorf("%s 압축을 풀지 못했습니다: %w", target, err)
		}
	}

	pkgs, err := c.parsePackages(raw)
	if err != nil {
		return nil, err
	}

	ix := &Index{
		Origin:      rel.Get("Origin"),
		Label:       rel.Get("Label"),
		Suite:       rel.Get("Suite"),
		Codename:    rel.Get("Codename"),
		Component:   c.cfg.Component,
		Arch:        c.cfg.Arch,
		RepoURL:     c.cfg.RepoURL,
		FetchedAt:   time.Now(),
		Signer:      signer,
		Packages:    pkgs,
		OtherArches: otherArches(rel.Get("Architectures"), c.cfg.Arch),
	}
	if err := c.saveIndex(ix); err != nil {
		return nil, err
	}
	return ix, nil
}

// parsePackages는 Packages 파일을 패키지 목록으로 바꿉니다.
// 같은 이름이 여러 버전으로 실려 있으면 가장 높은 버전만 남깁니다.
func (c *Client) parsePackages(raw []byte) (map[string]*Package, error) {
	paras, err := deb.ParseParagraphs(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("패키지 목록을 해석하지 못했습니다: %w", err)
	}

	out := map[string]*Package{}
	for _, p := range paras {
		name := p.Get("Package")
		if name == "" {
			continue
		}
		arch := p.Get("Architecture")
		// 목록 자체가 아키텍처별로 나뉘어 있지만, 섞여 들어온 항목이
		// 있어도 설치되지 않도록 한 번 더 확인한다.
		if arch != c.cfg.Arch && arch != "all" {
			continue
		}
		pkg := &Package{
			Name:         name,
			Version:      p.Get("Version"),
			Architecture: arch,
			Filename:     p.Get("Filename"),
			Size:         p.GetInt64("Size"),
			SHA256:       p.Get("SHA256"),
			Section:      p.Get("Section"),
			Homepage:     p.Get("Homepage"),
			Maintainer:   p.Get("Maintainer"),
			Description:  p.Get("Description"),
			Depends:      deb.ParseDepends(p.Get("Depends")),
		}
		if pkg.Filename == "" || pkg.SHA256 == "" {
			// 파일 위치나 해시가 없으면 안전하게 설치할 수 없다.
			continue
		}
		if prev, ok := out[name]; ok && deb.CompareVersions(pkg.Version, prev.Version) <= 0 {
			continue
		}
		out[name] = pkg
	}
	return out, nil
}

// Load는 캐시에 저장된 목록을 읽습니다.
func (c *Client) Load() (*Index, error) {
	b, err := os.ReadFile(c.indexCacheFile())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoIndex
		}
		return nil, err
	}
	var ix Index
	if err := json.Unmarshal(b, &ix); err != nil {
		return nil, fmt.Errorf("패키지 목록 캐시가 손상되었습니다. rpt update를 다시 실행하십시오: %w", err)
	}
	if ix.Arch != c.cfg.Arch || ix.RepoURL != c.cfg.RepoURL {
		return nil, ErrNoIndex
	}
	return &ix, nil
}

// ErrNoIndex는 사용할 수 있는 패키지 목록 캐시가 없다는 뜻입니다.
var ErrNoIndex = fmt.Errorf("패키지 목록이 없습니다")

// saveIndex는 목록을 캐시 파일에 원자적으로 씁니다.
func (c *Client) saveIndex(ix *Index) error {
	if err := os.MkdirAll(c.cfg.ListsDir(), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(ix, "", "  ")
	if err != nil {
		return err
	}
	dst := c.indexCacheFile()
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// Download는 deb 파일을 캐시로 내려받고 SHA256을 확인합니다.
// 이미 받아 둔 파일이 해시까지 맞으면 다시 받지 않습니다.
func (c *Client) Download(p *Package) (string, bool, error) {
	if err := os.MkdirAll(c.cfg.CacheDir(), 0o755); err != nil {
		return "", false, err
	}
	dst := filepath.Join(c.cfg.CacheDir(), path.Base(p.Filename))

	if b, err := os.ReadFile(dst); err == nil {
		if verifySHA256(b, p.SHA256) == nil {
			return dst, true, nil
		}
		// 내용이 다르면 받다 만 파일이므로 버린다.
		_ = os.Remove(dst)
	}

	req, err := c.newRequest(c.url(p.Filename))
	if err != nil {
		return "", false, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("%s 응답: %s", path.Base(p.Filename), resp.Status)
	}

	tmp, err := os.CreateTemp(c.cfg.CacheDir(), ".rpt-dl-*")
	if err != nil {
		return "", false, err
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()

	src, sum := sha256Reader(resp.Body)
	if _, err := io.Copy(tmp, src); err != nil {
		return "", false, fmt.Errorf("%s 내려받기에 실패했습니다: %w", path.Base(p.Filename), err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		return "", false, err
	}
	if err := tmp.Close(); err != nil {
		return "", false, err
	}
	if got := sum(); !strings.EqualFold(got, p.SHA256) {
		return "", false, fmt.Errorf("%s 무결성 검증에 실패했습니다 (기대 %s, 실제 %s)",
			path.Base(p.Filename), p.SHA256, got)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return "", false, err
	}
	return dst, false, nil
}

// url은 저장소 기준 주소에 하위 경로를 붙입니다.
func (c *Client) url(parts ...string) string {
	base := strings.TrimSuffix(c.cfg.RepoURL, "/")
	return base + "/" + path.Join(parts...)
}

// newRequest는 저장소 요청을 만듭니다.
func (c *Client) newRequest(rawURL string) (*http.Request, error) {
	if _, err := url.Parse(rawURL); err != nil {
		return nil, fmt.Errorf("주소가 올바르지 않습니다 (%s): %w", rawURL, err)
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", config.UserAgent)
	return req, nil
}

// fetch는 작은 메타데이터 파일을 통째로 받아 옵니다.
func (c *Client) fetch(rawURL string) ([]byte, error) {
	req, err := c.newRequest(rawURL)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("응답 %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

// hashEntry는 Release의 해시 목록 한 줄입니다.
type hashEntry struct {
	hash string
	size int64
}

// parseHashBlock은 Release의 SHA256 블록을 경로별 해시로 바꿉니다.
func parseHashBlock(block string) map[string]hashEntry {
	out := map[string]hashEntry{}
	for _, line := range strings.Split(block, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		size, _ := strconv.ParseInt(fields[1], 10, 64)
		out[fields[2]] = hashEntry{hash: fields[0], size: size}
	}
	return out
}

// otherArches는 이 기기에서 쓰지 않는 아키텍처 목록을 뽑습니다.
func otherArches(field, mine string) []string {
	var out []string
	for _, a := range strings.Fields(field) {
		if a != mine && a != "all" {
			out = append(out, a)
		}
	}
	return out
}
