package deb

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		// 같은 버전
		{"1.0", "1.0", 0},
		{"26.8.1-3", "26.8.1-3", 0},

		// ROKFOSS 저장소에서 실제로 쓰이는 형태
		{"26.7.5-13", "26.8.2-1", -1},
		{"26.8.1-3", "26.8.1-10", -1},
		{"26.07.22.3-1", "26.07.22.3-2", -1},

		// 숫자 구간은 자릿수가 아니라 값으로 비교한다
		{"1.9", "1.10", -1},
		{"1.010", "1.10", 0},

		// 에포크가 나머지를 압도한다
		{"1:0.1", "9.9", 1},
		{"0.1", "1:0.1", -1},

		// 물결표는 빈 문자열보다도 작다 (시험판 표기)
		{"1.0~rc1", "1.0", -1},
		{"1.0~rc1", "1.0~rc2", -1},
		{"1.0~", "1.0", -1},

		// 알파벳이 나머지 기호보다 작다
		{"1.0a", "1.0+", -1},

		// 리비전이 없으면 있는 쪽보다 작다
		{"1.0", "1.0-1", -1},
	}

	for _, tc := range tests {
		got := sign(CompareVersions(tc.a, tc.b))
		if got != tc.want {
			t.Errorf("CompareVersions(%q, %q) = %d, 기대값 %d", tc.a, tc.b, got, tc.want)
		}
		// 반대 방향도 일관되어야 한다.
		if rev := sign(CompareVersions(tc.b, tc.a)); rev != -tc.want {
			t.Errorf("CompareVersions(%q, %q) = %d, 기대값 %d (역방향)", tc.b, tc.a, rev, -tc.want)
		}
	}
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	default:
		return 0
	}
}

func TestParseDepends(t *testing.T) {
	deps := ParseDepends("libc6 (>= 2.34), rsync | rsync2, openssh-client:amd64")
	want := []string{"libc6", "rsync", "openssh-client"}
	if len(deps) != len(want) {
		t.Fatalf("의존성 %d개, 기대값 %d개: %+v", len(deps), len(want), deps)
	}
	for i, w := range want {
		if deps[i].Name != w {
			t.Errorf("deps[%d].Name = %q, 기대값 %q", i, deps[i].Name, w)
		}
	}
	if deps[0].Raw != "libc6 (>= 2.34)" {
		t.Errorf("Raw = %q, 원본 표기가 유지되어야 합니다", deps[0].Raw)
	}
	if got := ParseDepends(""); len(got) != 0 {
		t.Errorf("빈 필드는 의존성이 없어야 합니다: %+v", got)
	}
}
