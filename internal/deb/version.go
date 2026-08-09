package deb

import (
	"strconv"
	"strings"
)

// CompareVersions는 데비안 버전 두 개를 비교합니다.
// a가 작으면 음수, 같으면 0, 크면 양수를 돌려줍니다.
//
// 데비안 정책의 버전 비교 규칙을 따릅니다.
// 버전은 [epoch:]upstream[-revision] 형태이며, 각 조각은 숫자와 비숫자
// 부분을 번갈아 비교합니다. 비숫자 부분에서는 물결표(~)가 무엇보다도
// 작고(빈 문자열보다도 작습니다) 알파벳이 나머지 기호보다 작습니다.
func CompareVersions(a, b string) int {
	ae, au, ar := splitVersion(a)
	be, bu, br := splitVersion(b)

	if ae != be {
		if ae < be {
			return -1
		}
		return 1
	}
	if c := compareFragment(au, bu); c != 0 {
		return c
	}
	return compareFragment(ar, br)
}

// splitVersion은 버전을 에포크, 업스트림 버전, 데비안 리비전으로 나눕니다.
func splitVersion(v string) (epoch int, upstream, revision string) {
	v = strings.TrimSpace(v)
	if idx := strings.IndexByte(v, ':'); idx >= 0 {
		if n, err := strconv.Atoi(v[:idx]); err == nil {
			epoch = n
			v = v[idx+1:]
		}
	}
	if idx := strings.LastIndexByte(v, '-'); idx >= 0 {
		upstream, revision = v[:idx], v[idx+1:]
	} else {
		upstream, revision = v, ""
	}
	return epoch, upstream, revision
}

// compareFragment는 업스트림 버전이나 리비전 한 조각을 비교합니다.
func compareFragment(a, b string) int {
	for len(a) > 0 || len(b) > 0 {
		// 비숫자 구간을 문자 단위로 비교한다.
		i, j := 0, 0
		for i < len(a) && !isDigit(a[i]) {
			i++
		}
		for j < len(b) && !isDigit(b[j]) {
			j++
		}
		if c := compareNonDigit(a[:i], b[:j]); c != 0 {
			return c
		}
		a, b = a[i:], b[j:]

		// 숫자 구간은 앞의 0을 무시하고 수치로 비교한다.
		i, j = 0, 0
		for i < len(a) && isDigit(a[i]) {
			i++
		}
		for j < len(b) && isDigit(b[j]) {
			j++
		}
		na := strings.TrimLeft(a[:i], "0")
		nb := strings.TrimLeft(b[:j], "0")
		if len(na) != len(nb) {
			if len(na) < len(nb) {
				return -1
			}
			return 1
		}
		if na != nb {
			if na < nb {
				return -1
			}
			return 1
		}
		a, b = a[i:], b[j:]
	}
	return 0
}

// compareNonDigit은 비숫자 구간을 데비안 정렬 순서로 비교합니다.
func compareNonDigit(a, b string) int {
	for k := 0; k < len(a) || k < len(b); k++ {
		var ca, cb byte
		if k < len(a) {
			ca = a[k]
		}
		if k < len(b) {
			cb = b[k]
		}
		if ra, rb := orderRune(ca), orderRune(cb); ra != rb {
			if ra < rb {
				return -1
			}
			return 1
		}
	}
	return 0
}

// orderRune은 문자에 데비안 정렬 가중치를 매깁니다.
// 물결표는 빈 문자열보다도 작아야 하므로 음수를 줍니다.
func orderRune(c byte) int {
	switch {
	case c == 0:
		return 0
	case c == '~':
		return -1
	case isAlpha(c):
		return int(c)
	default:
		return int(c) + 256
	}
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isAlpha(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
