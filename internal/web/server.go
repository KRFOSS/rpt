// Package web는 rpt의 로컬 웹 대시보드를 제공합니다.
package web

import (
	"encoding/json"
	"fmt"
	"html/template"
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
	return s
}

// Addr는 서버가 바인드할 주소를 돌려줍니다.
func (s *Server) Addr() string { return s.addr }

// ListenAndServe는 서버를 시작합니다.
func (s *Server) ListenAndServe() error {
	server := &http.Server{
		Addr:           s.addr,
		Handler:        s.mux,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    30 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	return server.ListenAndServe()
}

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
	Installed      []installedRow
	Result         *commandResult
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
}

type commandResult struct {
	Command string
	OK      bool
	Lines   []string
	Error   string
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
	return data
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

func errorsIs(err, target error) bool { return err != nil && target != nil && (err == target || strings.Contains(err.Error(), target.Error())) }

const pageTemplate = `<!doctype html>
<html lang="ko">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="color-scheme" content="dark">
  <title>rpt 웹 대시보드</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/gh/orioncactus/pretendard@v1.3.9/dist/web/variable/pretendardvariable-dynamic-subset.css">
  <style>
    :root {
      --focus-ring: #06b6d4;
    }

    html {
      box-sizing: border-box;
    }

    *, *::before, *::after {
      box-sizing: inherit;
    }

    html, body {
      width: 100%;
      overflow-x: hidden;
    }

    * {
      font-family: "Pretendard Variable", Pretendard, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "Noto Sans", sans-serif;
    }

    body {
      background: #1a1a1a;
      color: #e5e5e5;
      margin: 0;
      padding: 0;
      position: relative;
      min-height: 100vh;
      display: flex;
      flex-direction: column;
    }

    :focus-visible {
      outline: 3px solid var(--focus-ring);
      outline-offset: 2px;
    }

    /* Navbar */
    .navbar {
      position: fixed;
      top: 0;
      left: 0;
      width: 100%;
      box-shadow: none !important;
      border: none !important;
      padding-bottom: 40px !important;
      z-index: 1000;
      background: linear-gradient(to bottom, rgba(0,0,0,0.8) 0%, rgba(0,0,0,0.65) 20%, rgba(0,0,0,0.50) 40%, rgba(0,0,0,0.35) 60%, rgba(0,0,0,0.20) 75%, rgba(0,0,0,0.10) 85%, rgba(0,0,0,0.04) 93%, rgba(0,0,0,0) 100%) !important;
      transition: transform 0.4s cubic-bezier(0.4,0,0.2,1), background 0.4s ease;
      padding: 23px !important;
      transform: translateY(0);
    }

    .navbar .container {
      display: flex;
      justify-content: space-between;
      align-items: center;
      flex-wrap: nowrap;
      width: 100%;
      max-width: 1200px;
      margin: 0 auto;
      padding: 0 1rem;
    }

    .navbar-brand img {
      height: 40px;
    }

    /* Main */
    .main-container {
      flex: 1;
      display: flex;
      flex-direction: column;
      align-items: center;
      padding: 140px 20px 80px;
      box-sizing: border-box;
    }

    .content-wrapper {
      max-width: 1200px;
      width: 100%;
      text-align: center;
    }

    .grid-2col {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 24px;
      margin-bottom: 32px;
    }

    .grid-2col .info-section {
      margin-bottom: 0;
    }

    h1 {
      font-size: 48px;
      font-weight: 700;
      color: #fff;
      margin-bottom: 0.5rem;
    }

    .subtitle {
      font-size: 20px;
      color: #fff;
      margin-bottom: 0.5rem;
    }

    .description {
      font-size: 16px;
      line-height: 1.5;
      color: #e5e5e5;
      margin-bottom: 2.5rem;
    }

    /* Info Section */
    .info-section {
      background-color: #242424;
      border-radius: 10px;
      padding: 20px;
      margin-bottom: 32px;
      text-align: left;
    }

    .info-section-title {
      font-size: 16px;
      font-weight: 700;
      color: #fff;
      margin: 0 0 16px 0;
      padding-bottom: 12px;
      border-bottom: 0.5px solid rgba(84,84,88,0.4);
    }

    .info-item {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 16px 0;
      border-bottom: 0.5px solid rgba(84,84,88,0.4);
    }

    .info-item:last-child {
      border-bottom: none;
      padding-bottom: 0;
    }

    .info-item:first-child {
      padding-top: 0;
    }

    .info-label {
      font-size: 15px;
      line-height: 20px;
      color: rgba(235,235,245,0.6);
      font-weight: 500;
      flex-shrink: 0;
      margin-right: 16px;
    }

    .info-value {
      font-size: 14px;
      line-height: 20px;
      color: #ffffff;
      font-weight: 400;
      text-align: right;
      font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Monaco, Consolas, monospace;
      word-break: break-all;
      white-space: pre-wrap;
      max-width: 75%;
    }

    /* Buttons */
    .actions {
      display: flex;
      gap: 12px;
      margin-bottom: 32px;
      justify-content: center;
      flex-wrap: wrap;
    }

    .button {
      background-color: #242424;
      color: #ffffff;
      border: 1px solid rgba(84,84,88,0.4);
      border-radius: 10px;
      padding: 12px 24px;
      font-size: 15px;
      font-weight: 600;
      text-decoration: none;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      cursor: pointer;
      transition: background-color 0.2s, transform 0.1s;
      user-select: none;
    }

    .button:hover {
      background-color: #2c2c2e;
    }

    .button:active {
      transform: scale(0.98);
    }

    .button-primary {
      background-color: #ffffff;
      color: #000000;
      border: none;
    }

    .button-primary:hover {
      background-color: rgba(255,255,255,0.85);
    }

    .button-sm {
      padding: 8px 16px;
      font-size: 13px;
      border-radius: 8px;
    }

    .button-danger {
      color: #ef4444;
      border-color: rgba(239,68,68,0.3);
    }

    .button-danger:hover {
      background-color: rgba(239,68,68,0.1);
    }

    .button-success {
      color: #34d399;
      border-color: rgba(52,211,153,0.3);
    }

    .button-success:hover {
      background-color: rgba(52,211,153,0.1);
    }

    /* Forms inside sections */
    .form-group {
      margin-bottom: 16px;
    }

    .form-group:last-child {
      margin-bottom: 0;
    }

    .form-label {
      display: block;
      font-size: 14px;
      color: rgba(235,235,245,0.6);
      font-weight: 500;
      margin-bottom: 8px;
    }

    .form-input {
      width: 100%;
      box-sizing: border-box;
      background: #1a1a1a;
      color: #e5e5e5;
      border: 1px solid rgba(84,84,88,0.4);
      border-radius: 8px;
      padding: 10px 12px;
      font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Monaco, Consolas, monospace;
      font-size: 14px;
      outline: none;
      transition: border-color 0.2s;
    }

    .form-input:focus {
      border-color: rgba(84,84,88,0.8);
    }

    textarea.form-input {
      min-height: 60px;
      resize: vertical;
    }

    .form-check {
      display: flex;
      align-items: center;
      cursor: pointer;
      gap: 8px;
      margin-top: 8px;
    }

    .form-check input[type="checkbox"],
    .form-check input[type="radio"] {
      width: 16px;
      height: 16px;
      accent-color: #06b6d4;
    }

    .form-check span {
      font-size: 14px;
      color: #e5e5e5;
    }

    .form-row {
      display: flex;
      gap: 12px;
      align-items: flex-end;
    }

    .form-row .form-group {
      flex: 1;
    }

    .form-divider {
      height: 1px;
      background: rgba(84,84,88,0.4);
      margin: 20px 0;
      border: none;
    }

    /* Result */
    .result-card {
      background-color: #242424;
      border-radius: 10px;
      padding: 20px;
      margin-bottom: 32px;
      text-align: left;
      border: 1px solid rgba(84,84,88,0.4);
    }

    .result-card.result-ok {
      border-color: rgba(52,211,153,0.5);
    }

    .result-card.result-err {
      border-color: rgba(239,68,68,0.5);
    }

    .result-title {
      font-size: 16px;
      font-weight: 700;
      margin: 0 0 12px 0;
    }

    .result-ok .result-title {
      color: #34d399;
    }

    .result-err .result-title {
      color: #ef4444;
    }

    .result-error {
      color: #ef4444;
      font-size: 14px;
      line-height: 1.6;
      margin-bottom: 12px;
      background: rgba(239,68,68,0.08);
      padding: 12px;
      border-radius: 8px;
      border: 1px solid rgba(239,68,68,0.2);
    }

    .result-pre {
      white-space: pre-wrap;
      margin: 0;
      font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Monaco, Consolas, monospace;
      font-size: 13px;
      line-height: 1.6;
      color: rgba(235,235,245,0.8);
      background: #1a1a1a;
      padding: 16px;
      border-radius: 8px;
      border: 1px solid rgba(84,84,88,0.4);
      max-height: 400px;
      overflow-y: auto;
    }

    /* Table */
    .pkg-table {
      width: 100%;
      border-collapse: collapse;
      font-size: 14px;
    }

    .pkg-table th {
      text-align: left;
      padding: 12px 12px;
      color: rgba(235,235,245,0.6);
      font-weight: 600;
      font-size: 13px;
      border-bottom: 1px solid rgba(84,84,88,0.4);
    }

    .pkg-table td {
      padding: 12px 12px;
      color: #e5e5e5;
      border-bottom: 0.5px solid rgba(84,84,88,0.25);
      vertical-align: top;
    }

    .pkg-table tr:last-child td {
      border-bottom: none;
    }

    .pkg-table .mono {
      font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Monaco, Consolas, monospace;
      font-size: 13px;
    }

    .badge {
      display: inline-block;
      padding: 3px 8px;
      border-radius: 6px;
      font-size: 12px;
      font-weight: 600;
    }

    .badge-upgrade {
      background: rgba(251,191,36,0.12);
      color: #fbbf24;
      border: 1px solid rgba(251,191,36,0.25);
    }

    .badge-auto {
      background: rgba(56,189,248,0.1);
      color: #38bdf8;
      border: 1px solid rgba(56,189,248,0.2);
    }

    .badge-manual {
      background: rgba(52,211,153,0.1);
      color: #34d399;
      border: 1px solid rgba(52,211,153,0.2);
    }

    .empty-text {
      text-align: center;
      color: rgba(235,235,245,0.4);
      padding: 32px 0;
      font-size: 15px;
    }

    .pkg-chips {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
    }

    .pkg-chip {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      background: #1a1a1a;
      border: 1px solid rgba(84,84,88,0.4);
      border-radius: 8px;
      padding: 8px 12px;
      font-size: 13px;
      color: #e5e5e5;
    }

    .pkg-chip-name {
      font-weight: 600;
      color: #fff;
      font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Monaco, Consolas, monospace;
    }

    .pkg-chip-ver {
      color: rgba(235,235,245,0.4);
      font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Monaco, Consolas, monospace;
      font-size: 12px;
    }

    .pkg-chip-actions {
      display: inline-flex;
      gap: 4px;
      margin-left: 4px;
    }

    .pkg-chip-btn {
      background: none;
      border: 1px solid rgba(84,84,88,0.4);
      border-radius: 5px;
      padding: 2px 8px;
      font-size: 11px;
      font-weight: 600;
      cursor: pointer;
      transition: background-color 0.2s, transform 0.1s;
    }

    .pkg-chip-btn:hover {
      transform: scale(0.97);
    }

    .pkg-chip-btn:active {
      transform: scale(0.95);
    }

    .pkg-chip-btn.remove {
      color: #fbbf24;
      border-color: rgba(251,191,36,0.3);
    }

    .pkg-chip-btn.remove:hover {
      background: rgba(251,191,36,0.1);
    }

    .pkg-chip-btn.purge {
      color: #ef4444;
      border-color: rgba(239,68,68,0.3);
    }

    .pkg-chip-btn.purge:hover {
      background: rgba(239,68,68,0.1);
    }

    .footer-text {
      font-size: 14px;
      color: #aaa;
      margin-top: 2rem;
      text-align: center;
    }

    @media (prefers-reduced-motion: reduce) {
      * {
        animation: none !important;
        transition: none !important;
      }
    }
  </style>
</head>
<body>
  <nav class="navbar">
    <div class="container">
      <a class="navbar-brand" href="/">
        <img loading="eager" decoding="async" src="https://cdn.krfoss.org/web/ROKFOSS.png" alt="ROKFOSS">
      </a>
    </div>
  </nav>

  <main class="main-container">
    <div class="content-wrapper">
      <h1>RPT</h1>
      <p class="subtitle">ROKFOSS 패키지 관리자</p>
      <p class="description">CLI 명령을 웹에서 실행합니다. update, install, remove, purge, upgrade, autoremove, list, search, show, clean, autoclean, relink 을 모두 지원합니다.</p>

      {{if .Result}}
      <div class="result-card {{if .Result.OK}}result-ok{{else}}result-err{{end}}">
        <h3 class="result-title">결과: {{.Result.Command}}</h3>
        {{if .Result.Error}}<div class="result-error">{{.Result.Error}}</div>{{end}}
        {{if .Result.Lines}}<pre class="result-pre">{{range .Result.Lines}}{{.}}
{{end}}</pre>{{end}}
      </div>
      {{end}}

      <!-- 시스템 상태 -->
      <div class="info-section">
        <h3 class="info-section-title">시스템 상태</h3>
        <div class="info-item">
          <span class="info-label">버전</span>
          <span class="info-value">{{.Version}}</span>
        </div>
        <div class="info-item">
          <span class="info-label">갱신 시각</span>
          <span class="info-value">{{.IndexFetchedAt}}</span>
        </div>
      </div>

      <!-- 기본 명령 -->
      <div class="actions">
        <form method="post" action="/command" style="display:contents;"><input type="hidden" name="cmd" value="update"><button type="submit" class="button button-primary">update</button></form>
        <form method="post" action="/command" style="display:contents;"><input type="hidden" name="cmd" value="upgrade"><button type="submit" class="button button-success">upgrade</button></form>
        <form method="post" action="/command" style="display:contents;"><input type="hidden" name="cmd" value="autoremove"><button type="submit" class="button">autoremove</button></form>
        <form method="post" action="/command" style="display:contents;"><input type="hidden" name="cmd" value="clean"><button type="submit" class="button">clean</button></form>
        <form method="post" action="/command" style="display:contents;"><input type="hidden" name="cmd" value="autoclean"><button type="submit" class="button">autoclean</button></form>
        <form method="post" action="/command" style="display:contents;"><input type="hidden" name="cmd" value="relink"><button type="submit" class="button">relink</button></form>
        <form method="post" action="/command" style="display:contents;"><input type="hidden" name="cmd" value="version"><button type="submit" class="button button-sm">version</button></form>
        <form method="post" action="/command" style="display:contents;"><input type="hidden" name="cmd" value="help"><button type="submit" class="button button-sm">help</button></form>
      </div>

      <div class="grid-2col">
        <!-- 설치 -->
        <div class="info-section">
          <h3 class="info-section-title">설치</h3>
          <form method="post" action="/command">
            <input type="hidden" name="cmd" value="install">
            <div class="form-group">
              <label class="form-label">패키지 이름</label>
              <input type="text" name="names" class="form-input" placeholder="krfs-rport">
            </div>
            <div class="form-group">
              <label class="form-check"><input type="checkbox" name="reinstall"><span>이미 설치된 경우 다시 설치</span></label>
            </div>
            <div class="form-group">
              <button type="submit" class="button button-primary" style="width:100%;">install</button>
            </div>
          </form>

          <hr class="form-divider">

          <form method="post" action="/command">
            <input type="hidden" name="cmd" value="upgrade">
            <div class="form-group">
              <label class="form-label">패키지 이름 (비우면 전체 업그레이드)</label>
              <input type="text" name="names" class="form-input" placeholder="krfs-rport">
            </div>
            <div class="form-group">
              <button type="submit" class="button button-success" style="width:100%;">upgrade</button>
            </div>
          </form>
        </div>

        <!-- 제거 -->
        <div class="info-section">
          <h3 class="info-section-title">제거</h3>
          {{if .Installed}}
          <div class="pkg-chips">
            {{range .Installed}}
            <div class="pkg-chip">
              <span class="pkg-chip-name">{{.Name}}</span>
              <span class="pkg-chip-ver">{{.Version}}</span>
              <span class="pkg-chip-actions">
                <form method="post" action="/command" style="display:inline;"><input type="hidden" name="cmd" value="remove"><input type="hidden" name="names" value="{{.Name}}"><button type="submit" class="pkg-chip-btn remove">remove</button></form>
                <form method="post" action="/command" style="display:inline;"><input type="hidden" name="cmd" value="purge"><input type="hidden" name="names" value="{{.Name}}"><button type="submit" class="pkg-chip-btn purge">purge</button></form>
              </span>
            </div>
            {{end}}
          </div>
          {{else}}
          <p class="empty-text">설치된 패키지가 없습니다.</p>
          {{end}}
        </div>
      </div>

      <div class="grid-2col">
        <!-- 조회 -->
        <div class="info-section">
          <h3 class="info-section-title">조회</h3>
          <form method="post" action="/command">
            <input type="hidden" name="cmd" value="list">
            <div class="form-group">
              <label class="form-check"><input type="checkbox" name="installed"><span>설치된 패키지만</span></label>
              <label class="form-check"><input type="checkbox" name="upgradable"><span>업그레이드 가능한 패키지만</span></label>
            </div>
            <div class="form-group">
              <button type="submit" class="button" style="width:100%;">list</button>
            </div>
          </form>

          <hr class="form-divider">

          <form method="post" action="/command">
            <input type="hidden" name="cmd" value="search">
            <div class="form-group">
              <label class="form-label">검색어</label>
              <input type="text" name="q" class="form-input" placeholder="rport">
            </div>
            <div class="form-group">
              <button type="submit" class="button" style="width:100%;">search</button>
            </div>
          </form>
        </div>

        <!-- 패키지 상세 -->
        <div class="info-section">
          <h3 class="info-section-title">패키지 상세</h3>
          <form method="post" action="/command">
            <input type="hidden" name="cmd" value="show">
            <div class="form-group">
              <label class="form-label">패키지 이름</label>
              <input type="text" name="names" class="form-input" placeholder="krfs-rport">
            </div>
            <div class="form-group">
              <button type="submit" class="button" style="width:100%;">show</button>
            </div>
          </form>
        </div>
      </div>

      <!-- 설치된 패키지 -->
      <div class="info-section">
        <h3 class="info-section-title">설치된 패키지</h3>
        {{if .Installed}}
        <div style="overflow-x: auto;">
          <table class="pkg-table">
            <thead>
              <tr><th>패키지</th><th>버전</th><th>상태</th><th>설명</th><th>설치 시각</th></tr>
            </thead>
            <tbody>
              {{range .Installed}}
              <tr>
                <td style="font-weight:600;color:#fff;">{{.Name}}</td>
                <td class="mono">{{.Version}}</td>
                <td>{{if .Upgradable}}<span class="badge badge-upgrade">업그레이드 가능</span>{{else}}<span class="badge badge-manual">설치됨</span>{{end}}</td>
                <td>{{if .Description}}{{.Description}}{{else}}<span style="color:rgba(235,235,245,0.3);">-</span>{{end}}</td>
                <td class="mono">{{.InstalledAt}}</td>
              </tr>
              {{end}}
            </tbody>
          </table>
        </div>
        {{else}}
        <p class="empty-text">설치된 패키지가 없습니다.</p>
        {{end}}
      </div>

      <p class="footer-text">rpt {{.Version}} · {{.Now}}</p>
    </div>
  </main>
</body>
</html>`