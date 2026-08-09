// Package deb는 dpkg 없이 데비안 패키지를 읽고 푸는 기능을 제공합니다.
//
// deb는 ar 아카이브 안에 control.tar.* 와 data.tar.* 가 들어 있는 구조라
// 외부 명령 없이 Go만으로 처리할 수 있습니다. ROKFOSS 저장소는 zstd로
// 압축하지만 gzip, xz, bzip2, 무압축도 함께 지원합니다.
package deb

import (
	"archive/tar"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// Archive는 열려 있는 deb 패키지 하나입니다.
type Archive struct {
	// Control은 control 파일의 필드들입니다.
	Control Paragraph
	// Conffiles는 설정 파일로 표시된 경로 집합입니다. 재설치나 업그레이드
	// 때 사용자가 고친 내용을 덮어쓰지 않기 위해 씁니다.
	Conffiles map[string]bool
	// Scripts는 postinst 같은 관리자 스크립트의 이름입니다.
	// rpt는 이 스크립트를 실행하지 않고 목록만 알려 줍니다.
	Scripts []string

	data []byte // data.tar.* 압축 해제 전 원본
	comp string // data 멤버의 확장자
}

// Open은 deb 파일을 열어 제어 정보를 읽습니다.
func Open(filename string) (*Archive, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	members, err := readAr(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(filename), err)
	}

	a := &Archive{Conffiles: map[string]bool{}}
	var controlRaw []byte
	var controlComp string
	for _, m := range members {
		switch {
		case strings.HasPrefix(m.Name, "control.tar"):
			controlRaw = m.Data
			controlComp = strings.TrimPrefix(m.Name, "control.tar")
		case strings.HasPrefix(m.Name, "data.tar"):
			a.data = m.Data
			a.comp = strings.TrimPrefix(m.Name, "data.tar")
		}
	}
	if controlRaw == nil {
		return nil, fmt.Errorf("%s: control.tar 멤버가 없습니다", filepath.Base(filename))
	}
	if a.data == nil {
		return nil, fmt.Errorf("%s: data.tar 멤버가 없습니다", filepath.Base(filename))
	}
	if err := a.readControl(controlRaw, controlComp); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(filename), err)
	}
	return a, nil
}

// readControl은 control.tar에서 control, conffiles, 관리자 스크립트를 읽습니다.
func (a *Archive) readControl(raw []byte, comp string) error {
	tr, closer, err := openTar(raw, comp)
	if err != nil {
		return err
	}
	defer closer()

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("control.tar를 읽지 못했습니다: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		name := path.Base(path.Clean("/" + strings.TrimPrefix(hdr.Name, "./")))
		switch name {
		case "control":
			paras, err := ParseParagraphs(tr)
			if err != nil {
				return err
			}
			if len(paras) == 0 {
				return fmt.Errorf("control 파일이 비어 있습니다")
			}
			a.Control = paras[0]
		case "conffiles":
			b, err := io.ReadAll(tr)
			if err != nil {
				return fmt.Errorf("conffiles를 읽지 못했습니다: %w", err)
			}
			for _, line := range strings.Split(string(b), "\n") {
				if p := strings.TrimSpace(line); p != "" {
					a.Conffiles[normalizePath(p)] = true
				}
			}
		case "preinst", "postinst", "prerm", "postrm":
			a.Scripts = append(a.Scripts, name)
		}
	}
	if a.Control == nil {
		return fmt.Errorf("control 파일이 없습니다")
	}
	return nil
}

// Name은 패키지 이름입니다.
func (a *Archive) Name() string { return a.Control.Get("Package") }

// Version은 패키지 버전입니다.
func (a *Archive) Version() string { return a.Control.Get("Version") }

// Architecture는 패키지 아키텍처입니다.
func (a *Archive) Architecture() string { return a.Control.Get("Architecture") }

// ExtractResult는 압축 해제 결과입니다.
type ExtractResult struct {
	// Files는 설치된 파일의 루트 기준 상대 경로입니다.
	Files []string
	// Dirs는 새로 만든 디렉터리의 루트 기준 상대 경로입니다.
	// 제거할 때 빈 디렉터리를 정리하는 데 씁니다.
	Dirs []string
	// KeptConffiles는 기존 내용을 지키느라 덮어쓰지 않은 설정 파일입니다.
	KeptConffiles []string
}

// Extract는 data.tar를 설치 루트 아래에 풉니다.
//
// keepConffiles가 참이면 이미 존재하는 설정 파일은 그대로 둡니다.
// 경로가 설치 루트를 벗어나면 오류로 처리해 시스템 파일을 건드리지
// 않도록 막습니다.
func (a *Archive) Extract(root string, keepConffiles bool) (*ExtractResult, error) {
	tr, closer, err := openTar(a.data, a.comp)
	if err != nil {
		return nil, err
	}
	defer closer()

	// 심링크를 타고 설치 루트 밖으로 나가는지 판단하려면 루트 자신의
	// 실제 경로를 알아야 합니다.
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		realRoot = root
	}

	res := &ExtractResult{}
	seenDirs := map[string]bool{}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return res, fmt.Errorf("data.tar를 읽지 못했습니다: %w", err)
		}

		rel := normalizePath(hdr.Name)
		if rel == "" || rel == "." {
			continue
		}
		target, err := safeJoin(root, rel)
		if err != nil {
			return res, err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := ensureParent(realRoot, target); err != nil {
				return res, fmt.Errorf("%s: %w", rel, err)
			}
			if err := os.MkdirAll(target, fs(hdr.Mode, 0o755)); err != nil {
				return res, fmt.Errorf("디렉터리를 만들지 못했습니다 (%s): %w", rel, err)
			}
			if err := checkInside(realRoot, target); err != nil {
				return res, fmt.Errorf("%s: %w", rel, err)
			}
			if !seenDirs[rel] {
				seenDirs[rel] = true
				res.Dirs = append(res.Dirs, rel)
			}

		case tar.TypeReg:
			if keepConffiles && a.Conffiles[rel] {
				if _, err := os.Lstat(target); err == nil {
					// 사용자가 고쳤을 수 있는 설정 파일은 건드리지 않는다.
					res.KeptConffiles = append(res.KeptConffiles, rel)
					res.Files = append(res.Files, rel)
					continue
				}
			}
			if err := ensureParent(realRoot, target); err != nil {
				return res, fmt.Errorf("%s: %w", rel, err)
			}
			if err := writeFile(target, tr, fs(hdr.Mode, 0o644)); err != nil {
				return res, fmt.Errorf("파일을 쓰지 못했습니다 (%s): %w", rel, err)
			}
			res.Files = append(res.Files, rel)

		case tar.TypeSymlink:
			if err := ensureParent(realRoot, target); err != nil {
				return res, fmt.Errorf("%s: %w", rel, err)
			}
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return res, fmt.Errorf("심링크를 만들지 못했습니다 (%s): %w", rel, err)
			}
			res.Files = append(res.Files, rel)

		case tar.TypeLink:
			linkRel := normalizePath(hdr.Linkname)
			linkTarget, err := safeJoin(root, linkRel)
			if err != nil {
				return res, err
			}
			if err := ensureParent(realRoot, target); err != nil {
				return res, fmt.Errorf("%s: %w", rel, err)
			}
			_ = os.Remove(target)
			if err := os.Link(linkTarget, target); err != nil {
				// 하드링크가 안 되면 심링크로 대체한다.
				if err := os.Symlink(linkTarget, target); err != nil {
					return res, fmt.Errorf("하드링크를 만들지 못했습니다 (%s): %w", rel, err)
				}
			}
			res.Files = append(res.Files, rel)

		default:
			// 장치 노드나 FIFO는 ROKFOSS 패키지에 없으며 건너뜁니다.
			continue
		}
	}
	return res, nil
}

// writeFile은 내용을 임시 파일에 쓴 뒤 제자리로 옮깁니다.
// 실행 중인 바이너리를 덮어쓸 때 "text file busy"를 피하려면
// 잘라 쓰기가 아니라 교체가 필요합니다.
func writeFile(target string, r io.Reader, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(target), ".rpt-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()

	if _, err := io.Copy(tmp, r); err != nil {
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, target)
}

// openTar는 압축 형식에 맞는 tar 리더를 엽니다.
func openTar(raw []byte, comp string) (*tar.Reader, func(), error) {
	br := bytes.NewReader(raw)
	noop := func() {}

	switch comp {
	case "", ".tar":
		return tar.NewReader(br), noop, nil
	case ".gz":
		zr, err := gzip.NewReader(br)
		if err != nil {
			return nil, noop, fmt.Errorf("gzip 압축을 풀지 못했습니다: %w", err)
		}
		return tar.NewReader(zr), func() { zr.Close() }, nil
	case ".zst":
		zr, err := zstd.NewReader(br)
		if err != nil {
			return nil, noop, fmt.Errorf("zstd 압축을 풀지 못했습니다: %w", err)
		}
		return tar.NewReader(zr), zr.Close, nil
	case ".xz":
		xr, err := xz.NewReader(br)
		if err != nil {
			return nil, noop, fmt.Errorf("xz 압축을 풀지 못했습니다: %w", err)
		}
		return tar.NewReader(xr), noop, nil
	case ".bz2":
		return tar.NewReader(bzip2.NewReader(br)), noop, nil
	default:
		return nil, noop, fmt.Errorf("지원하지 않는 압축 형식입니다: tar%s", comp)
	}
}

// normalizePath는 tar 항목 이름을 루트 기준 상대 경로로 정규화합니다.
func normalizePath(name string) string {
	name = strings.TrimPrefix(name, "./")
	name = path.Clean("/" + name)
	return strings.TrimPrefix(name, "/")
}

// ensureParent는 대상의 상위 디렉터리를 만들되, 그 경로가 심링크를 타고
// 설치 루트 밖으로 빠져나가지 않는지 확인합니다.
//
// 경로 문자열만 보는 검사는 아카이브가 먼저 바깥을 가리키는 심링크를 만들고
// 그 아래에 파일을 쓰는 수법을 잡지 못하므로, 실제 경로까지 풀어서 봅니다.
func ensureParent(realRoot, target string) error {
	parent := filepath.Dir(target)

	// 아직 없는 디렉터리는 풀어 볼 수 없으니, 존재하는 가장 깊은 조상을 먼저 검사한다.
	anc := parent
	for {
		if _, err := os.Lstat(anc); err == nil {
			break
		}
		next := filepath.Dir(anc)
		if next == anc {
			break
		}
		anc = next
	}
	if err := checkInside(realRoot, anc); err != nil {
		return err
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("상위 디렉터리를 만들지 못했습니다: %w", err)
	}
	return checkInside(realRoot, parent)
}

// checkInside는 경로의 실제 위치가 설치 루트 안인지 확인합니다.
func checkInside(realRoot, p string) error {
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	rel, err := filepath.Rel(realRoot, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("설치 루트 밖을 가리키는 경로입니다 (%s)", real)
	}
	return nil
}

// safeJoin은 상대 경로를 설치 루트에 붙이되 루트를 벗어나지 못하게 합니다.
func safeJoin(root, rel string) (string, error) {
	full := filepath.Join(root, filepath.FromSlash(rel))
	r, err := filepath.Rel(root, full)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("패키지에 설치 루트를 벗어나는 경로가 있습니다: %s", rel)
	}
	return full, nil
}

// fs는 tar 헤더의 모드를 파일 모드로 바꿉니다. 값이 비었으면 기본값을 씁니다.
func fs(mode int64, def os.FileMode) os.FileMode {
	m := os.FileMode(mode).Perm()
	if m == 0 {
		return def
	}
	return m
}
