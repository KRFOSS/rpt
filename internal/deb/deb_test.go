package deb

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseParagraphs(t *testing.T) {
	input := `Package: krfs-rport
Version: 26.8.1-3
Depends: libc6
Description: Encrypted reverse port forwarding tool
 첫째 줄
 .
 셋째 줄

Package: dsync
Version: 26.8.2-1
`
	paras, err := ParseParagraphs(strings.NewReader(input))
	if err != nil {
		t.Fatalf("해석 실패: %v", err)
	}
	if len(paras) != 2 {
		t.Fatalf("문단 %d개, 기대값 2", len(paras))
	}
	if got := paras[0].Get("Package"); got != "krfs-rport" {
		t.Errorf("Package = %q", got)
	}
	// 필드 이름은 대소문자를 가리지 않아야 한다.
	if got := paras[0].Get("VERSION"); got != "26.8.1-3" {
		t.Errorf("VERSION = %q", got)
	}
	wantDesc := "Encrypted reverse port forwarding tool\n첫째 줄\n\n셋째 줄"
	if got := paras[0].Get("Description"); got != wantDesc {
		t.Errorf("Description = %q, 기대값 %q", got, wantDesc)
	}
	if got := ShortDescription(paras[0].Get("Description")); got != "Encrypted reverse port forwarding tool" {
		t.Errorf("ShortDescription = %q", got)
	}
	if got := paras[1].Get("Package"); got != "dsync" {
		t.Errorf("두 번째 문단 Package = %q", got)
	}
}

func TestOpenAndExtract(t *testing.T) {
	control := "Package: demo\nVersion: 1.0-1\nArchitecture: amd64\nDescription: 시험용\n"
	controlTar := makeTarGz(t, []tarEntry{
		{name: "./control", body: control, mode: 0o644},
		{name: "./conffiles", body: "/etc/demo/demo.conf\n", mode: 0o644},
		{name: "./postinst", body: "#!/bin/sh\nexit 0\n", mode: 0o755},
	})
	dataTar := makeTarGz(t, []tarEntry{
		{name: "./usr/", dir: true, mode: 0o755},
		{name: "./usr/bin/", dir: true, mode: 0o755},
		{name: "./usr/bin/demo", body: "바이너리", mode: 0o755},
		{name: "./etc/demo/", dir: true, mode: 0o755},
		{name: "./etc/demo/demo.conf", body: "key=value\n", mode: 0o600},
		{name: "./usr/bin/demo-link", link: "demo"},
	})

	path := writeDeb(t, controlTar, dataTar, ".gz")
	a, err := Open(path)
	if err != nil {
		t.Fatalf("deb 열기 실패: %v", err)
	}
	if a.Name() != "demo" || a.Version() != "1.0-1" || a.Architecture() != "amd64" {
		t.Errorf("control 정보가 잘못 읽혔습니다: %s %s %s", a.Name(), a.Version(), a.Architecture())
	}
	if !a.Conffiles["etc/demo/demo.conf"] {
		t.Errorf("conffiles 목록이 잘못되었습니다: %+v", a.Conffiles)
	}
	if len(a.Scripts) != 1 || a.Scripts[0] != "postinst" {
		t.Errorf("관리자 스크립트 목록이 잘못되었습니다: %+v", a.Scripts)
	}

	root := t.TempDir()
	res, err := a.Extract(root, true)
	if err != nil {
		t.Fatalf("압축 해제 실패: %v", err)
	}
	if !contains(res.Files, "usr/bin/demo") || !contains(res.Files, "etc/demo/demo.conf") {
		t.Errorf("설치 파일 목록이 잘못되었습니다: %+v", res.Files)
	}
	if b, err := os.ReadFile(filepath.Join(root, "usr/bin/demo")); err != nil || string(b) != "바이너리" {
		t.Errorf("파일 내용이 다릅니다: %v %q", err, b)
	}
	if fi, err := os.Stat(filepath.Join(root, "usr/bin/demo")); err != nil || fi.Mode().Perm() != 0o755 {
		t.Errorf("실행 권한이 유지되지 않았습니다: %v", err)
	}
	if dest, err := os.Readlink(filepath.Join(root, "usr/bin/demo-link")); err != nil || dest != "demo" {
		t.Errorf("심링크가 잘못되었습니다: %v %q", err, dest)
	}

	// 두 번째 설치에서는 사용자가 고친 설정 파일을 지켜야 한다.
	confPath := filepath.Join(root, "etc/demo/demo.conf")
	if err := os.WriteFile(confPath, []byte("사용자수정\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	res2, err := a.Extract(root, true)
	if err != nil {
		t.Fatalf("재설치 실패: %v", err)
	}
	if !contains(res2.KeptConffiles, "etc/demo/demo.conf") {
		t.Errorf("설정 파일이 보존 목록에 없습니다: %+v", res2.KeptConffiles)
	}
	if b, _ := os.ReadFile(confPath); string(b) != "사용자수정\n" {
		t.Errorf("설정 파일이 덮어써졌습니다: %q", b)
	}
}

// 상대 경로로 위를 파고드는 항목은 루트 기준으로 정규화되어 무력화됩니다.
func TestExtractNeutralizesDotDot(t *testing.T) {
	control := "Package: evil\nVersion: 1.0\nArchitecture: amd64\n"
	controlTar := makeTarGz(t, []tarEntry{{name: "./control", body: control, mode: 0o644}})
	dataTar := makeTarGz(t, []tarEntry{
		{name: "../../etc/passwd", body: "탈취", mode: 0o644},
	})

	a, err := Open(writeDeb(t, controlTar, dataTar, ".gz"))
	if err != nil {
		t.Fatalf("deb 열기 실패: %v", err)
	}
	root := t.TempDir()
	res, err := a.Extract(root, true)
	if err != nil {
		t.Fatalf("압축 해제 실패: %v", err)
	}
	if !contains(res.Files, "etc/passwd") {
		t.Fatalf("경로가 루트 안으로 정규화되지 않았습니다: %+v", res.Files)
	}
	if _, err := os.Stat(filepath.Join(root, "etc/passwd")); err != nil {
		t.Errorf("파일이 설치 루트 안에 있어야 합니다: %v", err)
	}
	// 루트 바깥에는 아무것도 생기면 안 된다.
	if _, err := os.Lstat(filepath.Join(filepath.Dir(root), "etc")); err == nil {
		t.Error("설치 루트 바깥에 파일이 만들어졌습니다")
	}
}

// 아카이브가 바깥을 가리키는 심링크를 먼저 만들고 그 아래에 파일을 쓰는
// 수법은 거부해야 합니다.
func TestExtractRejectsSymlinkEscape(t *testing.T) {
	outside := t.TempDir()
	control := "Package: evil\nVersion: 1.0\nArchitecture: amd64\n"
	controlTar := makeTarGz(t, []tarEntry{{name: "./control", body: control, mode: 0o644}})
	dataTar := makeTarGz(t, []tarEntry{
		{name: "./sneak", link: outside},
		{name: "./sneak/pwned", body: "탈취", mode: 0o644},
	})

	a, err := Open(writeDeb(t, controlTar, dataTar, ".gz"))
	if err != nil {
		t.Fatalf("deb 열기 실패: %v", err)
	}
	root := t.TempDir()
	if _, err := a.Extract(root, true); err == nil {
		t.Fatal("심링크를 통한 설치 루트 탈출을 막지 못했습니다")
	}
	if _, err := os.Stat(filepath.Join(outside, "pwned")); err == nil {
		t.Error("설치 루트 바깥에 파일이 만들어졌습니다")
	}
}

type tarEntry struct {
	name string
	body string
	link string
	dir  bool
	mode int64
}

func makeTarGz(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Mode: e.mode}
		switch {
		case e.dir:
			hdr.Typeflag = tar.TypeDir
		case e.link != "":
			hdr.Typeflag = tar.TypeSymlink
			hdr.Linkname = e.link
		default:
			hdr.Typeflag = tar.TypeReg
			hdr.Size = int64(len(e.body))
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if hdr.Typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// writeDeb는 ar 컨테이너로 감싼 임시 deb 파일을 만듭니다.
func writeDeb(t *testing.T, controlTar, dataTar []byte, ext string) string {
	t.Helper()
	var buf bytes.Buffer
	buf.WriteString(arMagic)
	writeArMember(&buf, "debian-binary", []byte("2.0\n"))
	writeArMember(&buf, "control.tar"+ext, controlTar)
	writeArMember(&buf, "data.tar"+ext, dataTar)

	path := filepath.Join(t.TempDir(), "demo.deb")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeArMember(buf *bytes.Buffer, name string, data []byte) {
	fmt.Fprintf(buf, "%-16s%-12d%-6d%-6d%-8s%-10d`\n",
		name, 0, 0, 0, "100644", len(data))
	buf.Write(data)
	// 멤버는 짝수 경계에 맞춰야 한다.
	if len(data)%2 == 1 {
		buf.WriteByte('\n')
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
