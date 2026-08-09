// rpt(ROKFOSS APT)는 apt 가 없는 환경에서 ROKFOSS 저장소의
// 데비안 패키지를 설치하고 관리하는 도구입니다.
//
// 시놀로지 DSM 처럼 dpkg 와 apt 가 모두 없는 시스템을 대상으로 하며,
// 자체 설치 루트와 자체 상태 파일만 사용해 시스템 패키지 관리자와
// 절대 충돌하지 않습니다.
package main

import (
	"os"

	"github.com/krfoss/rpt/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
