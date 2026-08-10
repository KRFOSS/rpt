// rpt(ROKFOSS APT)는 ROKFOSS 저장소의 데비안 패키지를 설치하고
// 관리하는 도구입니다.
//
// 시놀로지 DSM 처럼 dpkg 와 apt 가 아예 없는 시스템은 물론, dpkg 가 있는
// 배포판에서도 씁니다. 어느 쪽이든 자체 설치 루트와 자체 상태 파일만
// 사용하므로 시스템 패키지 관리자와 섞이지 않습니다.
package main

import (
	"os"

	"github.com/krfoss/rpt/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
