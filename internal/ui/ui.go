// Package ui는 rpt의 터미널 출력을 담당합니다.
//
// 모든 사용자 대상 문구는 한국어이며, 색은 상태 구분에 필요한
// 최소한만 사용합니다.
package ui

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

var useColor = term.IsTerminal(int(os.Stderr.Fd())) && os.Getenv("NO_COLOR") == ""

const (
	reset  = "\033[0m"
	dim    = "\033[2m"
	bold   = "\033[1m"
	red    = "\033[31m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
)

func paint(code, s string) string {
	if !useColor {
		return s
	}
	return code + s + reset
}

// Step은 작업 단계 시작을 알립니다.
func Step(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", paint(cyan+bold, "::"), fmt.Sprintf(format, a...))
}

// Detail은 단계에 딸린 부가 정보를 들여쓰기해 출력합니다.
func Detail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "   %s\n", paint(dim, fmt.Sprintf(format, a...)))
}

// Info는 일반 알림을 출력합니다.
func Info(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "   %s\n", fmt.Sprintf(format, a...))
}

// Warn은 경고를 출력합니다. 작업은 계속 진행됩니다.
func Warn(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", paint(yellow+bold, "경고"), fmt.Sprintf(format, a...))
}

// Error는 오류를 출력합니다.
func Error(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", paint(red+bold, "오류"), fmt.Sprintf(format, a...))
}

// Out은 명령의 실제 결과를 표준 출력으로 내보냅니다.
// 다른 도구로 파이프될 수 있으므로 색을 입히지 않습니다.
func Out(format string, a ...any) {
	fmt.Fprintf(os.Stdout, format+"\n", a...)
}

// HumanSize는 바이트 수를 읽기 쉬운 단위로 바꿉니다.
func HumanSize(n int64) string {
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

// Indent는 여러 줄 텍스트의 각 줄 앞에 접두사를 붙입니다.
func Indent(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}
