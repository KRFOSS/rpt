package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/krfoss/rpt/internal/pkgmgr"
	"github.com/krfoss/rpt/internal/repo"
	"github.com/krfoss/rpt/internal/ui"
	webui "github.com/krfoss/rpt/internal/web"
)

// webForegroundEnv가 붙은 프로세스가 실제로 요청을 받는 쪽입니다.
// 사용자가 실행한 프로세스는 이 값을 넣어 자식을 띄우고 곧 끝납니다.
const webForegroundEnv = "RPT_WEB_FOREGROUND"

// webPIDFile은 상태 디렉터리에 남기는 실행 기록입니다.
// 이것이 있어야 나중에 어느 프로세스를 멈춰야 할지 알 수 있습니다.
const webPIDFile = "web.pid"

const (
	// webStartWait는 띄운 서버가 응답할 때까지 기다리는 시간입니다.
	webStartWait = 5 * time.Second
	// webShutdownWait는 서버가 처리 중인 요청을 마치도록 주는 시간입니다.
	webShutdownWait = 15 * time.Second
	// webStopWait는 멈추라고 이른 뒤 실제로 끝나기를 기다리는 시간입니다.
	// 서버가 스스로 정리하는 시간보다 길어야 중간에 잘리지 않습니다.
	webStopWait = 20 * time.Second
)

// cmdWeb은 로컬 웹 대시보드를 다룹니다.
func cmdWeb(m *pkgmgr.Manager, args []string, opts options) error {
	if len(args) > 1 {
		return fmt.Errorf("web 명령에 인자가 너무 많습니다: %s", strings.Join(args[1:], " "))
	}
	sub := ""
	if len(args) == 1 {
		sub = strings.ToLower(args[0])
	}

	switch sub {
	case "":
		return startWeb(m, opts)

	case "stop":
		stopped, err := stopWeb(m)
		if err != nil {
			return err
		}
		if !stopped {
			ui.Info("실행 중인 웹 대시보드가 없습니다.")
		}
		return nil

	case "restart":
		if _, err := stopWeb(m); err != nil {
			return err
		}
		return startWeb(m, opts)

	default:
		return fmt.Errorf("알 수 없는 web 하위 명령입니다: %s (쓸 수 있는 것: stop, restart)", sub)
	}
}

// startWeb은 대시보드를 띄웁니다.
func startWeb(m *pkgmgr.Manager, opts options) error {
	addr := m.Cfg.WebAddr
	if opts.Addr != "" {
		addr = opts.Addr
	}
	if err := m.LoadIndex(); err != nil && !errors.Is(err, repo.ErrNoIndex) {
		return err
	}
	if os.Getenv(webForegroundEnv) != "" {
		return serveWeb(m, addr)
	}
	return spawnWeb(m, addr)
}

// serveWeb은 실제로 요청을 받는 쪽입니다.
//
// 주소를 먼저 잡고 나서 실행 기록을 남깁니다. 주소를 잡지 못하면 기록도
// 남지 않으므로, 남아 있는 기록은 언제나 실제로 뜬 서버를 가리킵니다.
func serveWeb(m *pkgmgr.Manager, addr string) error {
	srv := webui.New(m, addr)
	ln, err := srv.Listen()
	if err != nil {
		return fmt.Errorf("웹 서버 시작 실패: %w", err)
	}

	pidPath := webPIDPath(m)
	if err := writeWebPID(pidPath, os.Getpid(), addr); err != nil {
		ui.Warn("실행 기록을 남기지 못해 rpt web stop 을 쓸 수 없습니다 (%s): %v", pidPath, err)
	} else {
		defer func() { _ = os.Remove(pidPath) }()
	}

	// 멈추라는 신호를 받으면 처리 중인 요청을 마친 뒤에 닫습니다.
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	drained := make(chan struct{})
	go func() {
		<-sigc
		ctx, cancel := context.WithTimeout(context.Background(), webShutdownWait)
		defer cancel()
		_ = srv.Shutdown(ctx)
		close(drained)
	}()

	// Serve는 Shutdown이 불린 즉시 돌아옵니다. 여기서 그대로 끝내 버리면
	// 처리 중이던 요청이 잘리므로, 정리가 끝날 때까지 기다립니다.
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("웹 서버가 멈췄습니다: %w", err)
	}
	<-drained
	return nil
}

// spawnWeb은 요청을 받을 프로세스를 따로 띄우고 응답을 확인합니다.
func spawnWeb(m *pkgmgr.Manager, addr string) error {
	pidPath := webPIDPath(m)
	if pid, running, err := readWebPID(pidPath); err == nil && webProcessAlive(pid) {
		return fmt.Errorf("웹 대시보드가 이미 %s 에서 돌고 있습니다 (프로세스 %d). "+
			"rpt web restart 로 다시 띄우거나 rpt web stop 으로 멈추십시오", running, pid)
	}

	// 남이 쥐고 있는 주소인지 먼저 봅니다. 자식을 띄운 뒤에 알면
	// 시작했다는 말과 실패했다는 말이 뒤섞여 나옵니다.
	probe, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("%s 를 쓸 수 없습니다: %w", addr, err)
	}
	_ = probe.Close()

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("웹 서버 실행 파일을 찾지 못했습니다: %w", err)
	}
	proc, err := os.StartProcess(exe, []string{exe, "web"}, &os.ProcAttr{
		Env:   webChildEnv(addr),
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		Sys:   &syscall.SysProcAttr{Setsid: true},
	})
	if err != nil {
		return fmt.Errorf("웹 서버를 백그라운드로 시작하지 못했습니다: %w", err)
	}

	if err := waitWebReady(proc, addr); err != nil {
		return err
	}
	ui.Step("웹 대시보드를 시작했습니다: http://%s", addr)
	ui.Detail("프로세스 %d · 멈추려면 rpt web stop", proc.Pid)
	ui.Detail("상태 API: http://%s/api/status", addr)
	return nil
}

// webChildEnv는 자식에게 넘길 환경을 만듭니다.
//
// 같은 이름을 그냥 덧붙이면 환경 변수가 두 벌이 되어 어느 쪽이 쓰일지
// 알 수 없으므로, 먼저 걷어내고 새로 답니다.
func webChildEnv(addr string) []string {
	parent := os.Environ()
	out := make([]string, 0, len(parent)+2)
	for _, kv := range parent {
		if strings.HasPrefix(kv, webForegroundEnv+"=") || strings.HasPrefix(kv, "RPT_WEB_ADDR=") {
			continue
		}
		out = append(out, kv)
	}
	return append(out, webForegroundEnv+"=1", "RPT_WEB_ADDR="+addr)
}

// waitWebReady는 띄운 서버가 실제로 응답할 때까지 기다립니다.
//
// 예전에는 자식을 띄우자마자 시작했다고 알렸습니다. 자식이 주소를 잡지
// 못하고 죽어도 성공 메시지가 먼저 나오는 바람에, 뒤늦게 따라붙는 오류와
// 순서가 뒤집혀 보였습니다.
func waitWebReady(proc *os.Process, addr string) error {
	exited := make(chan struct{})
	go func() {
		_, _ = proc.Wait()
		close(exited)
	}()

	deadline := time.After(webStartWait)
	tick := time.NewTicker(80 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-exited:
			return fmt.Errorf("웹 서버가 시작하자마자 끝났습니다 (%s)", addr)
		case <-deadline:
			return fmt.Errorf("웹 서버가 %s 안에 응답하지 않았습니다 (%s)", webStartWait, addr)
		case <-tick.C:
			conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
			if err == nil {
				_ = conn.Close()
				return nil
			}
		}
	}
}

// stopWeb은 돌고 있는 대시보드를 멈춥니다.
// 멈출 것이 없으면 false를 돌려주며, 이것은 오류가 아닙니다.
func stopWeb(m *pkgmgr.Manager) (bool, error) {
	pidPath := webPIDPath(m)
	pid, addr, err := readWebPID(pidPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("실행 기록을 읽지 못했습니다 (%s): %w", pidPath, err)
	}
	if !webProcessAlive(pid) {
		// 기록만 남고 프로세스는 이미 없는 경우입니다.
		_ = os.Remove(pidPath)
		return false, nil
	}

	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		return false, fmt.Errorf("웹 서버를 멈추지 못했습니다 (프로세스 %d): %w", pid, err)
	}
	if !waitProcessGone(pid, webStopWait) {
		ui.Warn("처리 중인 일이 %s 안에 끝나지 않아 강제로 종료합니다 (프로세스 %d)", webStopWait, pid)
		_ = syscall.Kill(pid, syscall.SIGKILL)
		if !waitProcessGone(pid, 3*time.Second) {
			return false, fmt.Errorf("웹 서버를 끝내지 못했습니다 (프로세스 %d)", pid)
		}
	}

	_ = os.Remove(pidPath)
	ui.Step("웹 대시보드를 멈췄습니다 (%s, 프로세스 %d)", addr, pid)
	return true, nil
}

// webPIDPath는 실행 기록 파일의 경로입니다.
func webPIDPath(m *pkgmgr.Manager) string {
	return filepath.Join(m.Cfg.StatePath(), webPIDFile)
}

// writeWebPID는 실행 기록을 남깁니다.
// 첫 줄이 프로세스 번호, 둘째 줄이 듣고 있는 주소입니다.
func writeWebPID(path string, pid int, addr string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n%s\n", pid, addr)), 0o644)
}

// readWebPID는 실행 기록을 읽습니다.
func readWebPID(path string) (int, string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, "", err
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	pid, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return 0, "", fmt.Errorf("실행 기록을 알아볼 수 없습니다: %w", err)
	}
	addr := ""
	if len(lines) > 1 {
		addr = strings.TrimSpace(lines[1])
	}
	return pid, addr, nil
}

// webProcessAlive는 기록된 프로세스가 아직 살아 있는 rpt web 인지 봅니다.
//
// 프로세스 번호는 돌려쓰이므로 기록만 믿고 신호를 보내면, 그 번호를
// 물려받은 엉뚱한 프로세스를 죽일 수 있습니다.
func webProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if err := syscall.Kill(pid, 0); err != nil && !errors.Is(err, syscall.EPERM) {
		return false
	}
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		// 확인할 길이 없으면 살아 있다는 것까지만 믿습니다.
		return true
	}
	args := strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00")
	if len(args) < 2 {
		return false
	}
	return filepath.Base(args[0]) == selfName() && args[1] == "web"
}

// waitProcessGone은 프로세스가 사라질 때까지 기다립니다.
func waitProcessGone(pid int, wait time.Duration) bool {
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err != nil && !errors.Is(err, syscall.EPERM) {
			return true
		}
		time.Sleep(80 * time.Millisecond)
	}
	return false
}

// selfName은 지금 실행 중인 파일의 이름입니다.
func selfName() string {
	exe, err := os.Executable()
	if err != nil {
		return "rpt"
	}
	return filepath.Base(exe)
}
