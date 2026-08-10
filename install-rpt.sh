#!/usr/bin/env bash

set -euo pipefail

WORKDIR="${WORKDIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
BUILD_DIR="${BUILD_DIR:-$WORKDIR/.build}"
BIN_NAME="${BIN_NAME:-rpt}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
INSTALL_PATH="$INSTALL_DIR/$BIN_NAME"

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    printf '필수 명령이 없습니다: %s\n' "$1" >&2
    exit 1
  }
}

main() {
  if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
    printf '이 스크립트는 root 권한으로 실행해야 합니다.\n' >&2
    exit 1
  fi

  need_cmd go
  need_cmd install

  mkdir -p "$BUILD_DIR"
  printf '빌드 중: %s\n' "$WORKDIR"
  CGO_ENABLED=0 GOOS=linux GOARCH="${GOARCH:-$(go env GOARCH)}" \
    go build -trimpath -ldflags='-s -w' -o "$BUILD_DIR/$BIN_NAME" "$WORKDIR"

  install -m 755 "$BUILD_DIR/$BIN_NAME" "$INSTALL_PATH"
  printf '설치 완료: %s\n' "$INSTALL_PATH"
  printf '버전 확인: %s version\n' "$BIN_NAME"
}

main "$@"