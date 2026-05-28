#!/usr/bin/env bash
#!nix-shell -i bash -p bash go_1_26 git gnugrep coreutils
# shellcheck shell=bash
# shfmt: format with `shfmt -ln bash`; bash is the runtime shebang.
set -Eeuo pipefail

usage() {
  cat <<'EOF'
Usage: ./build.sh [options]

Build the lsx-server binary.

Options:
  --target OS/ARCH       Target pair, e.g. linux/amd64, darwin/arm64, windows/amd64
  --os OS                Target OS: linux, darwin, macos, windows
  --arch ARCH            Target arch: 386, amd64, x64, arm64, aarch64
  --out PATH             Output path
  --name NAME            Binary base name when --out is not set
  --version VERSION      Version string embedded into main.version
  --skip-tests           Generate assets and build without running tests
  --skip-checks          Generate assets and build without running checks
  --list-targets         Print supported linux/darwin/windows targets
  -h, --help             Show this help

Environment overrides:
  APP_NAME, OUT, VERSION, GOOS, GOARCH
EOF
}

die() {
  printf '%s\n' "$*" >&2
  exit 2
}

project_targets() {
  go tool dist list | grep -E '^(linux|darwin|windows)/(386|amd64|arm64)$'
}

version() {
  git describe --tags --always --dirty 2>/dev/null || printf 'dev'
}

normalize() {
  local os=${1,,} arch=${2,,}

  case "$os" in mac | macos | osx) os=darwin ;; win) os=windows ;; esac
  case "$arch" in x64 | x86_64) arch=amd64 ;; aarch64) arch=arm64 ;; x86 | i386 | i686) arch=386 ;; esac
  printf '%s/%s' "$os" "$arch"
}

split_target() {
  [[ $1 == */* ]] || die "target must be OS/ARCH: $1"
  TARGET_OS=${1%/*}
  TARGET_ARCH=${1#*/}
}

need_value() {
  [[ $# -ge 2 ]] || die "missing value for $1"
}

while (($#)); do
  case "$1" in
    --target)
      need_value "$@"
      split_target "$2"
      shift 2
      ;;
    --target=*)
      split_target "${1#*=}"
      shift
      ;;
    --os)
      need_value "$@"
      TARGET_OS=$2
      shift 2
      ;;
    --os=*)
      TARGET_OS=${1#*=}
      shift
      ;;
    --arch)
      need_value "$@"
      TARGET_ARCH=$2
      shift 2
      ;;
    --arch=*)
      TARGET_ARCH=${1#*=}
      shift
      ;;
    --out)
      need_value "$@"
      OUT=$2
      shift 2
      ;;
    --out=*)
      OUT=${1#*=}
      shift
      ;;
    --name)
      need_value "$@"
      APP_NAME=$2
      shift 2
      ;;
    --name=*)
      APP_NAME=${1#*=}
      shift
      ;;
    --version)
      need_value "$@"
      VERSION=$2
      shift 2
      ;;
    --version=*)
      VERSION=${1#*=}
      shift
      ;;
    --skip-tests)
      RUN_TESTS=0
      shift
      ;;
    --skip-checks)
      RUN_CHECKS=0
      RUN_TESTS=0
      shift
      ;;
    --list-targets)
      project_targets
      exit 0
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      usage >&2
      die "unknown option: $1"
      ;;
  esac
done

cd "$(dirname -- "${BASH_SOURCE[0]}")"

APP_NAME=${APP_NAME:-lsx-server}
TARGET_OS=${TARGET_OS:-${GOOS:-$(go env GOOS)}}
TARGET_ARCH=${TARGET_ARCH:-${GOARCH:-$(go env GOARCH)}}
VERSION=${VERSION:-$(version)}
RUN_TESTS=${RUN_TESTS:-1}
RUN_CHECKS=${RUN_CHECKS:-1}
export CGO_ENABLED=0
HOST_OS=$(go env GOHOSTOS)
HOST_ARCH=$(go env GOHOSTARCH)
TARGET=$(normalize "$TARGET_OS" "$TARGET_ARCH")
TARGET_OS=${TARGET%/*}
TARGET_ARCH=${TARGET#*/}

grep -Fx "$TARGET" <<<"$(project_targets)" >/dev/null ||
  die "unsupported target: $TARGET"

if [[ -z ${OUT:-} ]]; then
  suffix=
  [[ $TARGET_OS == windows ]] && suffix=.exe
  OUT="dist/${APP_NAME}_${TARGET_OS}_${TARGET_ARCH}${suffix}"
fi

printf 'Building Svelte browser assets...\n'
command -v npm >/dev/null ||
  die "npm is required to build browser assets"
npm --prefix web ci
npm --prefix web run build

if [[ $RUN_TESTS == 1 ]]; then
  if [[ $RUN_CHECKS == 1 ]]; then
    printf 'Checking formatting...\n'
    unformatted=$(gofmt -l $(find . -name '*.go' -not -path './vendor/*'))
    [[ -z $unformatted ]] || die "gofmt required for:\n$unformatted"

    printf 'Running go vet...\n'
    GOOS=$HOST_OS GOARCH=$HOST_ARCH go vet ./...
  fi

  printf 'Running tests...\n'
  GOOS=$HOST_OS GOARCH=$HOST_ARCH go test ./...

  if [[ $RUN_CHECKS == 1 ]]; then
    command -v golangci-lint >/dev/null ||
      die "golangci-lint is required for build checks; install it or pass --skip-checks"

    printf 'Running golangci-lint...\n'
    GOOS=$HOST_OS GOARCH=$HOST_ARCH golangci-lint run ./...

    printf 'Running modern Go lint checks...\n'
    GOOS=$HOST_OS GOARCH=$HOST_ARCH golangci-lint run \
      --enable=modernize \
      --enable=usestdlibvars \
      --enable=exptostd \
      --enable=prealloc \
      --enable=perfsprint \
      --enable=gocritic \
      ./...
  fi
fi

printf 'Building optimized %s binary: %s\n' "$TARGET" "$OUT"
mkdir -p "$(dirname -- "$OUT")"
GOOS=$TARGET_OS GOARCH=$TARGET_ARCH go build \
  -trimpath \
  -tags webdist \
  -ldflags "-s -w -X main.version=$VERSION" \
  -o "$OUT" \
  .

printf 'Built %s\n' "$OUT"
