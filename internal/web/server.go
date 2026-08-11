// Package web는 rpt의 로컬 웹 대시보드를 제공합니다.
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/krfoss/rpt/internal/config"
	"github.com/krfoss/rpt/internal/deb"
	"github.com/krfoss/rpt/internal/pkgmgr"
	"github.com/krfoss/rpt/internal/repo"
)

// Server는 로컬 웹 대시보드입니다.
type Server struct {
	mgr  *pkgmgr.Manager
	addr string
	tpl  *template.Template
	mux  *http.ServeMux
	srv  *http.Server
}

// New는 새 서버를 만듭니다.
func New(mgr *pkgmgr.Manager, addr string) *Server {
	s := &Server{mgr: mgr, addr: addr}
	s.tpl = template.Must(template.New("page").Parse(pageTemplate))
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/command", s.handleCommand)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/healthz", s.handleHealth)
	s.mux = mux
	s.srv = &http.Server{
		Addr:           addr,
		Handler:        mux,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    30 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	return s
}

// Addr는 서버가 바인드할 주소를 돌려줍니다.
func (s *Server) Addr() string { return s.addr }

// Listen은 주소만 먼저 잡습니다. 요청은 아직 받지 않습니다.
//
// 주소를 잡는 것과 요청을 받는 것을 나눠 두면, 띄운 쪽이 "주소를 잡는 데
// 성공했다"는 사실을 확인한 뒤에 다음 일을 할 수 있습니다.
func (s *Server) Listen() (net.Listener, error) {
	return net.Listen("tcp", s.addr)
}

// Serve는 잡아 둔 주소로 요청을 받기 시작합니다.
func (s *Server) Serve(ln net.Listener) error { return s.srv.Serve(ln) }

// ListenAndServe는 주소를 잡고 곧바로 요청을 받습니다.
func (s *Server) ListenAndServe() error {
	ln, err := s.Listen()
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

// Shutdown은 처리 중인 요청이 끝나기를 기다렸다가 서버를 닫습니다.
//
// 설치가 한창일 때 프로세스를 그냥 죽이면 무엇을 설치했는지가 상태
// 파일에 남지 않아 파일만 남고 기록이 사라집니다.
func (s *Server) Shutdown(ctx context.Context) error { return s.srv.Shutdown(ctx) }

// ServeHTTP는 Server를 http.Handler 로도 쓸 수 있게 합니다.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

type pageData struct {
	Now            string
	Version        string
	Addr           string
	Root           string
	StateRoot      string
	CacheRoot      string
	RepoURL        string
	Arch           string
	IndexAvailable bool
	IndexOrigin    string
	IndexSuite     string
	IndexComponent string
	IndexFetchedAt string
	PackageCount   int
	InstalledCount int
	// UpgradableCount는 새 버전이 나와 있는 설치 패키지 수입니다.
	UpgradableCount int
	Installed       []installedRow
	// Available은 저장소에서 고를 수 있는 패키지입니다. 설치 칸의
	// 고르기 목록을 채웁니다.
	Available []pkgOption
	// InstalledOptions는 업그레이드 칸의 고르기 목록입니다.
	InstalledOptions []pkgOption
	Result           *commandResult
}

// pkgOption은 고르기 목록에 오르는 패키지 하나입니다.
type pkgOption struct {
	Name       string
	Version    string
	Summary    string
	Installed  bool
	Upgradable bool
	// SearchKey는 목록에서 걸러낼 때 쓰는 소문자 문자열입니다.
	// 브라우저에서 매번 소문자로 바꾸지 않도록 미리 만들어 둡니다.
	SearchKey string
}

type installedRow struct {
	Name           string
	Version        string
	Section        string
	Description    string
	Auto           bool
	InstalledAt    string
	Conffiles      int
	Links          int
	SkippedScripts int
	Upgradable     bool

	// Summary와 InstalledAtText는 화면에만 쓰는 값입니다. JSON 응답의
	// description과 installed_at은 예전 형식 그대로 두어야 하므로
	// 표시용을 따로 둡니다.
	Summary         string `json:"-"`
	InstalledAtText string `json:"-"`
}

type commandResult struct {
	Command string   `json:"command"`
	OK      bool     `json:"ok"`
	Lines   []string `json:"lines"`
	Error   string   `json:"error"`
}

type statusResponse struct {
	Now            time.Time      `json:"now"`
	Version        string         `json:"version"`
	Addr           string         `json:"addr"`
	Root           string         `json:"root"`
	StateRoot      string         `json:"state_root"`
	CacheRoot      string         `json:"cache_root"`
	RepoURL        string         `json:"repo_url"`
	Arch           string         `json:"arch"`
	IndexAvailable bool           `json:"index_available"`
	IndexOrigin    string         `json:"index_origin,omitempty"`
	IndexSuite     string         `json:"index_suite,omitempty"`
	IndexComponent string         `json:"index_component,omitempty"`
	IndexFetchedAt *time.Time     `json:"index_fetched_at,omitempty"`
	PackageCount   int            `json:"package_count"`
	InstalledCount int            `json:"installed_count"`
	Installed      []installedRow `json:"installed"`
}

// handleIndex는 대시보드를 보여줍니다.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	s.render(w, nil)
}

// handleCommand는 모든 CLI 명령을 웹에서 실행합니다.
func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	result := s.executeCommand(r)

	// partial=1 은 화면을 통째로 다시 그리지 말고 결과만 달라는 뜻입니다.
	// 브라우저가 이것으로 결과를 받아 모달에 띄우므로, 명령을 실행해도
	// 페이지가 새로 열리며 맨 위로 튀지 않습니다.
	if r.FormValue("partial") == "1" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(result)
		return
	}
	s.render(w, result)
}

// handleStatus는 상태 정보를 JSON 으로 돌려줍니다.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(s.buildStatusResponse())
}

// handleHealth는 간단한 생존 확인입니다.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) render(w http.ResponseWriter, result *commandResult) {
	if err := s.ensureIndex(); err != nil && !errorsIs(err, repo.ErrNoIndex) {
		result = &commandResult{Command: "dashboard", OK: false, Error: err.Error()}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tpl.Execute(w, s.buildPageData(result)); err != nil {
		http.Error(w, fmt.Sprintf("렌더링 실패: %v", err), http.StatusInternalServerError)
	}
}

func (s *Server) buildPageData(result *commandResult) pageData {
	status := s.buildStatusResponse()
	data := pageData{
		Now:            status.Now.Format("2006-01-02 15:04:05"),
		Version:        status.Version,
		Addr:           status.Addr,
		Root:           status.Root,
		StateRoot:      status.StateRoot,
		CacheRoot:      status.CacheRoot,
		RepoURL:        status.RepoURL,
		Arch:           status.Arch,
		IndexAvailable: status.IndexAvailable,
		IndexOrigin:    status.IndexOrigin,
		IndexSuite:     status.IndexSuite,
		IndexComponent: status.IndexComponent,
		IndexFetchedAt: "-",
		PackageCount:   status.PackageCount,
		InstalledCount: status.InstalledCount,
		Installed:      status.Installed,
		Result:         result,
	}
	if status.IndexAvailable && status.IndexFetchedAt != nil {
		data.IndexFetchedAt = status.IndexFetchedAt.Format("2006-01-02 15:04:05")
	}
	for _, row := range status.Installed {
		if row.Upgradable {
			data.UpgradableCount++
		}
	}
	data.Available = s.availableOptions()
	data.InstalledOptions = installedOptions(status.Installed)
	return data
}

// availableOptions는 저장소 목록에서 고를 수 있는 패키지를 추립니다.
// 목록을 아직 받지 않았으면 비어 있고, 그때는 화면이 직접 입력으로 돌아갑니다.
func (s *Server) availableOptions() []pkgOption {
	if s.mgr.Index == nil {
		return nil
	}
	names := s.mgr.Index.Names()
	out := make([]pkgOption, 0, len(names))
	for _, name := range names {
		p := s.mgr.Index.Packages[name]
		opt := pkgOption{
			Name:    p.Name,
			Version: p.Version,
			Summary: p.Summary(),
		}
		if inst, ok := s.mgr.DB.Get(name); ok {
			opt.Installed = true
			opt.Upgradable = deb.CompareVersions(p.Version, inst.Version) > 0
		}
		opt.SearchKey = strings.ToLower(opt.Name + " " + opt.Summary)
		out = append(out, opt)
	}
	return out
}

// installedOptions는 설치된 패키지를 고르기 목록 형태로 바꿉니다.
func installedOptions(rows []installedRow) []pkgOption {
	out := make([]pkgOption, 0, len(rows))
	for _, r := range rows {
		out = append(out, pkgOption{
			Name:       r.Name,
			Version:    r.Version,
			Summary:    r.Summary,
			Installed:  true,
			Upgradable: r.Upgradable,
			SearchKey:  strings.ToLower(r.Name + " " + r.Summary),
		})
	}
	return out
}

func (s *Server) buildStatusResponse() statusResponse {
	installed := s.installedRows()
	resp := statusResponse{
		Now:            time.Now(),
		Version:        config.Version,
		Addr:           s.addr,
		Root:           s.mgr.Cfg.Root,
		StateRoot:      s.mgr.Cfg.StateRoot,
		CacheRoot:      s.mgr.Cfg.CacheRoot,
		RepoURL:        s.mgr.Cfg.RepoURL,
		Arch:           s.mgr.Cfg.Arch,
		IndexAvailable: s.mgr.Index != nil,
		PackageCount:   0,
		InstalledCount: len(installed),
		Installed:      installed,
	}
	if s.mgr.Index != nil {
		resp.IndexOrigin = s.mgr.Index.Origin
		resp.IndexSuite = s.mgr.Index.Suite
		resp.IndexComponent = s.mgr.Index.Component
		resp.PackageCount = len(s.mgr.Index.Packages)
		resp.IndexFetchedAt = &s.mgr.Index.FetchedAt
	}
	return resp
}

func (s *Server) installedRows() []installedRow {
	items := s.mgr.DB.List()
	rows := make([]installedRow, 0, len(items))
	for _, item := range items {
		rows = append(rows, installedRow{
			Name:           item.Name,
			Version:        item.Version,
			Section:        item.Section,
			Description:    item.Description,
			Auto:           item.Auto,
			InstalledAt:    item.InstalledAt.Format(time.RFC3339),
			Conffiles:      len(item.Conffiles),
			Links:          len(item.Links),
			SkippedScripts: len(item.SkippedScripts),

			// 표에는 설명 첫 줄만 넣습니다. 전문을 그대로 넣으면
			// 여러 줄짜리 설명이 표를 무너뜨립니다.
			Summary:         deb.ShortDescription(item.Description),
			InstalledAtText: item.InstalledAt.Format("2006-01-02 15:04"),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	if s.mgr.Index != nil {
		for i := range rows {
			if p, ok := s.mgr.Index.Get(rows[i].Name); ok && deb.CompareVersions(p.Version, rows[i].Version) > 0 {
				rows[i].Upgradable = true
			}
		}
	}
	return rows
}

func (s *Server) ensureIndex() error {
	if s.mgr.Index != nil {
		return nil
	}
	ix, err := s.mgr.Client.Load()
	if err != nil {
		return err
	}
	s.mgr.Index = ix
	return nil
}

func (s *Server) executeCommand(r *http.Request) *commandResult {
	cmd := strings.ToLower(strings.TrimSpace(r.FormValue("cmd")))
	res := &commandResult{Command: cmd}
	appendf := func(format string, a ...any) { res.Lines = append(res.Lines, fmt.Sprintf(format, a...)) }
	appendLines := func(lines ...string) { res.Lines = append(res.Lines, lines...) }
	joinArgs := func(raw string) []string {
		raw = strings.ReplaceAll(raw, "\n", " ")
		raw = strings.ReplaceAll(raw, ",", " ")
		return strings.Fields(raw)
	}
	loadIndex := func() error { return s.ensureIndex() }
	rootNeeded := func() error {
		if os.Geteuid() != 0 {
			return fmt.Errorf("이 명령은 root 권한이 필요합니다 (sudo 로 다시 실행하십시오)")
		}
		return nil
	}

	switch cmd {
	case "version":
		res.OK = true
		appendf("rpt %s (ROKFOSS 패키지 관리자, %s)", config.Version, config.DebArch())
		return res
	case "help":
		res.OK = true
		appendLines(
			"rpt 웹 대시보드",
			"",
			"지원 명령: update, install, remove, purge, upgrade, autoremove, list, search, show, clean, autoclean, relink, version, help",
			"",
			"설치/제거/업그레이드/정리 명령은 root 권한이 필요합니다.",
		)
		return res
	case "update":
		if err := rootNeeded(); err != nil {
			res.Error = err.Error()
			return res
		}
		ix, err := s.mgr.Client.Update()
		if err != nil {
			res.Error = err.Error()
			return res
		}
		s.mgr.Index = ix
		res.OK = true
		appendf("저장소 목록을 갱신했습니다: %s", s.mgr.Cfg.RepoURL)
		if ix.Signer != nil {
			appendf("서명 확인: %s", ix.Signer.Identity)
		}
		appendf("%s / %s / %s", ix.Origin, ix.Suite, ix.Component)
		if len(ix.OtherArches) > 0 {
			appendf("%s 패키지 %d개 (%s 목록은 건너뜁니다)", ix.Arch, len(ix.Packages), strings.Join(ix.OtherArches, ", "))
		} else {
			appendf("%s 패키지 %d개", ix.Arch, len(ix.Packages))
		}
		return res
	}

	if cmd == "install" || cmd == "upgrade" || cmd == "remove" || cmd == "purge" || cmd == "search" || cmd == "show" || cmd == "list" || cmd == "clean" || cmd == "autoclean" {
		if err := loadIndex(); err != nil && !errorsIs(err, repo.ErrNoIndex) {
			res.Error = err.Error()
			return res
		}
	}

	switch cmd {
	case "install":
		if err := rootNeeded(); err != nil {
			res.Error = err.Error()
			return res
		}
		names := joinArgs(r.FormValue("names"))
		if len(names) == 0 {
			res.Error = "설치할 패키지 이름이 필요합니다"
			return res
		}
		plan, err := s.mgr.PlanInstall(names, r.FormValue("reinstall") != "")
		if err != nil {
			res.Error = err.Error()
			return res
		}
		for _, n := range plan.AlreadyCurrent {
			appendf("%s 는 이미 최신 버전입니다.", n)
		}
		for _, d := range plan.MissingDeps {
			appendf("의존성 %s 는 ROKFOSS 저장소에 없습니다. 시스템에 이미 있는지 확인하십시오.", d)
		}
		if plan.Empty() {
			appendLines("할 일이 없습니다.")
			res.OK = true
			return res
		}
		results, err := s.mgr.Execute(plan, nil)
		if err != nil {
			res.Error = err.Error()
			return res
		}
		appendLines("설치가 완료되었습니다.")
		for _, r := range results {
			appendf("%s %s", r.Name, r.Version)
			for _, p := range r.SkippedLinks {
				appendf("%s 는 이미 다른 파일이 있어 링크를 만들지 않았습니다: %s", r.Name, p)
			}
			for _, c := range r.KeptConffiles {
				appendf("설정 파일을 그대로 두었습니다: %s", filepath.Join(s.mgr.Cfg.Root, c))
			}
			if len(r.SkippedScripts) > 0 {
				appendf("%s 의 관리자 스크립트(%s)는 실행하지 않았습니다.", r.Name, strings.Join(r.SkippedScripts, ", "))
			}
		}
		res.OK = true
		return res

	case "remove", "purge":
		if err := rootNeeded(); err != nil {
			res.Error = err.Error()
			return res
		}
		names := joinArgs(r.FormValue("names"))
		if len(names) == 0 {
			res.Error = "지울 패키지 이름이 필요합니다"
			return res
		}
		plan := s.mgr.PlanRemove(names, cmd == "purge")
		for _, n := range plan.NotInstalled {
			appendf("%s 는 설치되어 있지 않습니다.", n)
		}
		for name, blockers := range plan.Blocked {
			appendf("%s 는 %s 가 의존하고 있어 지우지 않습니다.", name, strings.Join(blockers, ", "))
		}
		if plan.Empty() {
			appendLines("할 일이 없습니다.")
			res.OK = true
			return res
		}
		results, err := s.mgr.ExecuteRemove(plan, nil)
		if err != nil {
			res.Error = err.Error()
			return res
		}
		for _, rr := range results {
			appendf("%s 제거: 파일 %d개, 링크 %d개", rr.Name, rr.RemovedFiles, len(rr.RemovedLinks))
			for _, c := range rr.KeptConffiles {
				appendf("설정 파일을 남겼습니다: %s", filepath.Join(s.mgr.Cfg.Root, c))
			}
		}
		res.OK = true
		return res

	case "upgrade":
		if err := rootNeeded(); err != nil {
			res.Error = err.Error()
			return res
		}
		names := joinArgs(r.FormValue("names"))
		plan, err := s.mgr.PlanUpgrade(names)
		if err != nil {
			res.Error = err.Error()
			return res
		}
		if plan.Empty() {
			appendLines("모든 패키지가 최신입니다.")
			res.OK = true
			return res
		}
		results, err := s.mgr.Execute(plan, nil)
		if err != nil {
			res.Error = err.Error()
			return res
		}
		for _, r := range results {
			appendf("%s %s", r.Name, r.Version)
			for _, c := range r.KeptConffiles {
				appendf("설정 파일을 그대로 두었습니다: %s", filepath.Join(s.mgr.Cfg.Root, c))
			}
			if len(r.SkippedScripts) > 0 {
				appendf("%s 의 관리자 스크립트(%s)는 실행하지 않았습니다.", r.Name, strings.Join(r.SkippedScripts, ", "))
			}
		}
		res.OK = true
		return res

	case "autoremove":
		if err := rootNeeded(); err != nil {
			res.Error = err.Error()
			return res
		}
		names := s.mgr.AutoRemovable()
		if len(names) == 0 {
			appendLines("지울 것이 없습니다.")
			res.OK = true
			return res
		}
		plan := s.mgr.PlanRemove(names, false)
		results, err := s.mgr.ExecuteRemove(plan, nil)
		if err != nil {
			res.Error = err.Error()
			return res
		}
		for _, rr := range results {
			appendf("%s 제거: 파일 %d개, 링크 %d개", rr.Name, rr.RemovedFiles, len(rr.RemovedLinks))
		}
		res.OK = true
		return res

	case "list":
		installedOnly := r.FormValue("installed") != ""
		upgradableOnly := r.FormValue("upgradable") != ""
		if installedOnly {
			for _, rec := range s.mgr.DB.List() {
				mark := ""
				if rec.Auto {
					mark = " [자동]"
				}
				appendf("%s/%s %s%s", rec.Name, rec.Architecture, rec.Version, mark)
			}
			if len(res.Lines) == 0 {
				appendLines("설치된 패키지가 없습니다.")
			}
			res.OK = true
			return res
		}
		if err := loadIndex(); err != nil {
			if errorsIs(err, repo.ErrNoIndex) {
				res.Error = "패키지 목록이 없습니다. 먼저 update 를 실행하십시오."
			} else {
				res.Error = err.Error()
			}
			return res
		}
		for _, name := range s.mgr.Index.Names() {
			p := s.mgr.Index.Packages[name]
			inst, installed := s.mgr.DB.Get(name)
			upgradable := installed && deb.CompareVersions(p.Version, inst.Version) > 0
			if upgradableOnly && !upgradable {
				continue
			}
			switch {
			case upgradable:
				appendf("%s/%s %s [설치됨: %s, 업그레이드 가능]", p.Name, p.Architecture, p.Version, inst.Version)
			case installed:
				appendf("%s/%s %s [설치됨]", p.Name, p.Architecture, p.Version)
			default:
				appendf("%s/%s %s", p.Name, p.Architecture, p.Version)
			}
		}
		res.OK = true
		return res

	case "search":
		term := strings.ToLower(strings.TrimSpace(r.FormValue("q")))
		if term == "" {
			res.Error = "검색어가 필요합니다"
			return res
		}
		if err := loadIndex(); err != nil {
			if errorsIs(err, repo.ErrNoIndex) {
				res.Error = "패키지 목록이 없습니다. 먼저 update 를 실행하십시오."
			} else {
				res.Error = err.Error()
			}
			return res
		}
		found := 0
		for _, name := range s.mgr.Index.Names() {
			p := s.mgr.Index.Packages[name]
			hay := strings.ToLower(p.Name + " " + p.Description)
			if !strings.Contains(hay, term) {
				continue
			}
			found++
			mark := ""
			if _, ok := s.mgr.DB.Get(name); ok {
				mark = " [설치됨]"
			}
			appendf("%s/%s %s%s", p.Name, p.Architecture, p.Version, mark)
			if sum := p.Summary(); sum != "" {
				appendf("  %s", sum)
			}
		}
		if found == 0 {
			appendf("검색 결과가 없습니다: %s", term)
		}
		res.OK = true
		return res

	case "show":
		names := joinArgs(r.FormValue("names"))
		if len(names) == 0 {
			res.Error = "패키지 이름이 필요합니다"
			return res
		}
		if err := loadIndex(); err != nil {
			if errorsIs(err, repo.ErrNoIndex) {
				res.Error = "패키지 목록이 없습니다. 먼저 update 를 실행하십시오."
			} else {
				res.Error = err.Error()
			}
			return res
		}
		for i, name := range names {
			p, ok := s.mgr.Index.Get(name)
			if !ok {
				res.Error = fmt.Sprintf("패키지를 찾을 수 없습니다: %s", name)
				return res
			}
			if i > 0 {
				appendLines("")
			}
			appendf("패키지: %s", p.Name)
			appendf("버전: %s", p.Version)
			appendf("아키텍처: %s", p.Architecture)
			if p.Section != "" {
				appendf("분류: %s", p.Section)
			}
			if p.Maintainer != "" {
				appendf("관리자: %s", p.Maintainer)
			}
			if p.Homepage != "" {
				appendf("홈페이지: %s", p.Homepage)
			}
			appendf("크기: %s", humanSize(p.Size))
			if len(p.Depends) > 0 {
				var deps []string
				for _, d := range p.Depends {
					deps = append(deps, d.Raw)
				}
				appendf("의존성: %s", strings.Join(deps, ", "))
			}
			if inst, ok := s.mgr.DB.Get(name); ok {
				appendf("설치 상태: %s 설치됨 (%s)", inst.Version, inst.InstalledAt.Format("2006-01-02 15:04"))
				appendf("설치 파일: %d개", len(inst.Files))
			} else {
				appendLines("설치 상태: 설치되지 않음")
			}
			if p.Description != "" {
				appendLines("설명:")
				for _, line := range strings.Split(strings.TrimRight(p.Description, "\n"), "\n") {
					appendf("  %s", line)
				}
			}
		}
		res.OK = true
		return res

	case "clean":
		if err := rootNeeded(); err != nil {
			res.Error = err.Error()
			return res
		}
		clean, err := s.mgr.Clean()
		if err != nil {
			res.Error = err.Error()
			return res
		}
		if clean.Files == 0 {
			appendLines("정리할 캐시 파일이 없습니다.")
		} else {
			appendf("캐시 파일 %d개를 지웠습니다 (%s 확보).", clean.Files, humanSize(clean.Bytes))
		}
		res.OK = true
		return res

	case "autoclean":
		if err := rootNeeded(); err != nil {
			res.Error = err.Error()
			return res
		}
		if err := loadIndex(); err != nil {
			if errorsIs(err, repo.ErrNoIndex) {
				res.Error = "패키지 목록이 없어 옛 파일을 가릴 수 없습니다. 먼저 update 를 실행하십시오"
			} else {
				res.Error = err.Error()
			}
			return res
		}
		clean, err := s.mgr.AutoClean()
		if err != nil {
			res.Error = err.Error()
			return res
		}
		if clean.Files == 0 {
			appendLines("정리할 캐시 파일이 없습니다.")
		} else {
			appendf("캐시 파일 %d개를 지웠습니다 (%s 확보).", clean.Files, humanSize(clean.Bytes))
		}
		res.OK = true
		return res

	case "relink":
		if err := rootNeeded(); err != nil {
			res.Error = err.Error()
			return res
		}
		results, err := s.mgr.Relink()
		if err != nil {
			res.Error = err.Error()
			return res
		}
		if len(results) == 0 {
			appendLines("설치된 패키지가 없습니다.")
			res.OK = true
			return res
		}
		total := 0
		for _, rr := range results {
			total += len(rr.Created)
			for _, p := range rr.Skipped {
				appendf("%s: %s 는 이미 다른 파일이 있어 건너뛰었습니다.", rr.Package, p)
			}
		}
		appendf("심링크 %d개를 확인했습니다.", total)
		res.OK = true
		return res
	}

	res.Error = "처리되지 않은 명령입니다"
	return res
}

func (s *Server) loadIndexIfNeeded() error {
	if s.mgr.Index != nil {
		return nil
	}
	ix, err := s.mgr.Client.Load()
	if err != nil {
		return err
	}
	s.mgr.Index = ix
	return nil
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func errorsIs(err, target error) bool {
	return err != nil && target != nil && (err == target || strings.Contains(err.Error(), target.Error()))
}
