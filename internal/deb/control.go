package deb

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Paragraph는 데비안 제어 파일의 문단 하나입니다.
// 필드 이름은 대소문자를 구분하지 않으므로 소문자로 정규화해 담습니다.
type Paragraph map[string]string

// Get은 필드 값을 돌려줍니다. 없으면 빈 문자열입니다.
func (p Paragraph) Get(field string) string { return p[strings.ToLower(field)] }

// GetInt64는 정수 필드를 돌려줍니다. 값이 없거나 숫자가 아니면 0입니다.
func (p Paragraph) GetInt64(field string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(p.Get(field)), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// ParseParagraphs는 RFC822 형식의 제어 파일을 문단 단위로 나눕니다.
// Packages, control 파일이 모두 이 형식을 씁니다.
func ParseParagraphs(r io.Reader) ([]Paragraph, error) {
	var (
		out     []Paragraph
		cur     = Paragraph{}
		lastKey string
	)
	flush := func() {
		if len(cur) > 0 {
			out = append(out, cur)
			cur = Paragraph{}
		}
		lastKey = ""
	}

	sc := bufio.NewScanner(r)
	// Description 등이 길어질 수 있으므로 줄 버퍼를 넉넉히 잡는다.
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		// 공백이나 탭으로 시작하면 직전 필드의 이어지는 줄이다.
		if line[0] == ' ' || line[0] == '\t' {
			if lastKey == "" {
				continue
			}
			cont := strings.TrimSpace(line)
			// 데비안 규약상 홑점 한 글자는 빈 줄을 뜻한다.
			if cont == "." {
				cont = ""
			}
			cur[lastKey] += "\n" + cont
			continue
		}
		idx := strings.IndexByte(line, ':')
		if idx < 0 {
			return nil, fmt.Errorf("제어 파일 형식이 올바르지 않습니다: %q", line)
		}
		key := strings.ToLower(strings.TrimSpace(line[:idx]))
		val := strings.TrimSpace(line[idx+1:])
		cur[key] = val
		lastKey = key
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("제어 파일을 읽지 못했습니다: %w", err)
	}
	flush()
	return out, nil
}

// Dependency는 Depends 필드에서 뽑아낸 의존 대상 하나입니다.
type Dependency struct {
	// Name은 첫 번째 대안 패키지의 이름입니다.
	Name string
	// Raw는 대안과 버전 제약을 포함한 원본 표기입니다.
	Raw string
}

// ParseDepends는 Depends/Pre-Depends 필드를 해석합니다.
//
// "libc6, rsync (>= 3.0) | rsync2" 같은 표기에서 쉼표로 항목을 나누고,
// 각 항목은 첫 번째 대안만 취해 버전 제약을 떼어냅니다. ROKFOSS 저장소는
// 대안이나 버전 제약을 쓰지 않으므로 이 수준이면 충분합니다.
func ParseDepends(field string) []Dependency {
	var deps []Dependency
	for _, item := range strings.Split(field, ",") {
		raw := strings.TrimSpace(item)
		if raw == "" {
			continue
		}
		first := strings.TrimSpace(strings.Split(raw, "|")[0])
		if idx := strings.IndexByte(first, '('); idx >= 0 {
			first = strings.TrimSpace(first[:idx])
		}
		if idx := strings.IndexByte(first, ':'); idx >= 0 {
			// libc6:amd64 형태의 다중 아키텍처 한정자를 떼어낸다.
			first = strings.TrimSpace(first[:idx])
		}
		if first == "" {
			continue
		}
		deps = append(deps, Dependency{Name: first, Raw: raw})
	}
	return deps
}

// ShortDescription은 Description 필드의 첫 줄을 돌려줍니다.
func ShortDescription(desc string) string {
	if desc == "" {
		return ""
	}
	return strings.SplitN(desc, "\n", 2)[0]
}
