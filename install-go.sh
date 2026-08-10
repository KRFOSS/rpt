#!/usr/bin/env bash

set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-/usr/local/go}"
BIN_DIR="${BIN_DIR:-/usr/local/bin}"
GO_VERSION_URL="${GO_VERSION_URL:-https://go.dev/VERSION?m=text}"
GO_BASE_URL="${GO_BASE_URL:-https://go.dev/dl}"

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    printf '필수 명령이 없습니다: %s\n' "$1" >&2
    exit 1
  }
}

map_arch() {
  case "$(uname -m)" in
    x86_64|amd64)
      printf 'amd64'
      ;;
    aarch64|arm64)
      printf 'arm64'
      ;;
    i386|i686)
      printf '386'
      ;;
    armv6l|armv7l)
      printf 'armv6l'
      ;;
    ppc64le)
      printf 'ppc64le'
      ;;
    s390x)
      printf 's390x'
      ;;
    riscv64)
      printf 'riscv64'
      ;;
    *)
      printf '지원하지 않는 아키텍처입니다: %s\n' "$(uname -m)" >&2
      exit 1
      ;;
  esac
}

fetch() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- "$1"
  else
    printf 'curl 또는 wget 이 필요합니다.\n' >&2
    exit 1
  fi
}

download() {
  local url="$1"
  local dst="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fL "$url" -o "$dst"
  else
    wget -qO "$dst" "$url"
  fi
}

main() {
  if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
    printf '이 스크립트는 root 권한으로 실행해야 합니다.\n' >&2
    exit 1
  fi

  need_cmd tar
  need_cmd sha256sum
  need_cmd uname

  local version arch tarball version_url tarball_url checksum tmpdir tmpball tmpjson
  version="$(fetch "$GO_VERSION_URL" | head -n1 | tr -d '\r')"
  arch="$(map_arch)"
  tarball="go${version#go}.linux-${arch}.tar.gz"
  version_url="$GO_BASE_URL/$tarball"
  checksum_url="https://go.dev/dl/?mode=json&include=all"

  tmpdir=""
  cleanup() {
    if [[ -n "${tmpdir:-}" && -d "$tmpdir" ]]; then
      rm -rf "$tmpdir"
    fi
  }
  trap cleanup EXIT
  tmpdir="$(mktemp -d)"
  tmpball="$tmpdir/$tarball"
  tmpjson="$tmpdir/releases.json"

  printf 'Go 최신 버전: %s\n' "$version"
  printf '대상 아키텍처: %s\n' "$arch"
  printf '다운로드: %s\n' "$version_url"

  download "$version_url" "$tmpball"
  fetch "$checksum_url" > "$tmpjson"
  checksum="$(awk -v target="$tarball" '
    index($0, "\"filename\": \"" target "\"") {found=1}
    found && index($0, "\"sha256\": \"") {
      line = $0
      prefix = "\"sha256\": \""
      start = index(line, prefix)
      if (start == 0) {
        next
      }
      line = substr(line, start + length(prefix))
      end = index(line, "\"")
      if (end == 0) {
        next
      }
      print substr(line, 1, end - 1)
      exit
    }
  ' "$tmpjson")"
  if [[ -z "$checksum" ]]; then
    printf '체크섬을 찾지 못했습니다: %s\n' "$tarball" >&2
    exit 1
  fi
  printf '%s  %s\n' "$checksum" "$tmpball" | sha256sum -c -

  rm -rf "$INSTALL_DIR"
  tar -C "$(dirname "$INSTALL_DIR")" -xzf "$tmpball"

  ln -sf "$INSTALL_DIR/bin/go" "$BIN_DIR/go"
  ln -sf "$INSTALL_DIR/bin/gofmt" "$BIN_DIR/gofmt"

  printf '설치 완료: %s\n' "$INSTALL_DIR"
  printf '명령 경로: %s/go, %s/gofmt\n' "$BIN_DIR" "$BIN_DIR"
}

main "$@"