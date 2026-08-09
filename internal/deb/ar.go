package deb

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// arMagic은 ar 아카이브의 시작 표식입니다.
const arMagic = "!<arch>\n"

// arHeaderSize는 멤버 헤더의 고정 길이입니다.
const arHeaderSize = 60

// arMember는 ar 아카이브 안의 항목 하나입니다.
type arMember struct {
	Name string
	Size int64
	Data []byte
}

// readAr은 deb 컨테이너인 ar 아카이브를 통째로 읽습니다.
//
// deb는 debian-binary, control.tar.*, data.tar.* 세 멤버로만 이루어지고
// 각각 수 MB 수준이므로 메모리에 올려도 무리가 없습니다.
func readAr(r io.Reader) ([]arMember, error) {
	magic := make([]byte, len(arMagic))
	if _, err := io.ReadFull(r, magic); err != nil {
		return nil, fmt.Errorf("ar 헤더를 읽지 못했습니다: %w", err)
	}
	if string(magic) != arMagic {
		return nil, fmt.Errorf("ar 아카이브가 아닙니다 (deb 파일이 손상되었을 수 있습니다)")
	}

	var members []arMember
	hdr := make([]byte, arHeaderSize)
	for {
		if _, err := io.ReadFull(r, hdr); err != nil {
			if err == io.EOF {
				break
			}
			// 홀수 크기 멤버 뒤의 패딩만 남은 경우도 정상 종료로 본다.
			if err == io.ErrUnexpectedEOF {
				break
			}
			return nil, fmt.Errorf("ar 멤버 헤더를 읽지 못했습니다: %w", err)
		}
		if !bytes.Equal(hdr[58:60], []byte{'`', '\n'}) {
			return nil, fmt.Errorf("ar 멤버 헤더가 손상되었습니다")
		}

		name := strings.TrimSpace(string(hdr[0:16]))
		// GNU ar은 짧은 이름 뒤에 /를 붙인다.
		name = strings.TrimSuffix(name, "/")

		sizeField := strings.TrimSpace(string(hdr[48:58]))
		size, err := strconv.ParseInt(sizeField, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("ar 멤버 크기를 해석하지 못했습니다 (%q): %w", sizeField, err)
		}
		if size < 0 {
			return nil, fmt.Errorf("ar 멤버 크기가 올바르지 않습니다: %d", size)
		}

		data := make([]byte, size)
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, fmt.Errorf("ar 멤버 %q 본문을 읽지 못했습니다: %w", name, err)
		}
		members = append(members, arMember{Name: name, Size: size, Data: data})

		// 멤버는 짝수 바이트 경계에 정렬되므로 홀수면 패딩 1바이트를 버린다.
		if size%2 == 1 {
			var pad [1]byte
			if _, err := io.ReadFull(r, pad[:]); err != nil && err != io.EOF {
				return nil, fmt.Errorf("ar 패딩을 읽지 못했습니다: %w", err)
			}
		}
	}
	return members, nil
}
