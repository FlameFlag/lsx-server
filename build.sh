#!/usr/bin/env nix-shell
#! nix-shell -i bash -p bash go_1_26 git coreutils findutils nodejs_24 golangci-lint jdk21_headless gradle
# shellcheck shell=bash
# shfmt: format with `shfmt -ln bash`; bash is the runtime shebang.
set -Eeuo pipefail

usage() {
	cat <<'EOF'
Usage: ./build.sh [options]

Build release binaries.

Options:
  --target OS/ARCH       Target pair, e.g. linux/amd64, darwin/arm64, windows/amd64
  --os OS                Target OS: linux, darwin, macos, windows
  --arch ARCH            Target arch: 386, amd64, x64, arm64, aarch64
  --out PATH             Output path
  --name NAME            Binary base name when --out is not set
  --version VERSION      Version string embedded into main.version
  --skip-tests           Generate assets and build without running tests
  --skip-checks          Generate assets and build without running checks or tests
  --skip-tools           Build only the server binary
  --list-targets         Print supported linux/darwin/windows targets
  -h, --help             Show this help

Environment overrides:
  APP_NAME, OUT, VERSION, GOOS, GOARCH, CHECK_CGO_ENABLED, TOOL_CGO_ENABLED
EOF
}

die() {
	printf 'error: %s\n' "$*" >&2
	exit 2
}

info() {
	printf '%s\n' "$*"
}

need_value() {
	[[ $# -ge 2 && -n $2 ]] || die "missing value for $1"
}

require_command() {
	command -v "$1" >/dev/null || die "$1 is required$2"
}

version() {
	git describe --tags --always --dirty 2>/dev/null || printf 'dev'
}

project_targets() {
	local target

	go tool dist list | while IFS= read -r target; do
		case "$target" in
		linux/386 | linux/amd64 | linux/arm64 | darwin/386 | darwin/amd64 | darwin/arm64 | windows/386 | windows/amd64 | windows/arm64)
			printf '%s\n' "$target"
			;;
		esac
	done
}

normalize_target() {
	local os=${1,,} arch=${2,,}

	case "$os" in
	mac | macos | osx) os=darwin ;;
	win) os=windows ;;
	esac
	case "$arch" in
	x64 | x86_64) arch=amd64 ;;
	aarch64) arch=arm64 ;;
	x86 | i386 | i686) arch=386 ;;
	esac

	printf '%s/%s' "$os" "$arch"
}

split_target() {
	[[ $1 == */* ]] || die "target must be OS/ARCH: $1"
	TARGET_OS=${1%/*}
	TARGET_ARCH=${1#*/}
}

target_suffix() {
	[[ $1 == windows ]] && printf '.exe'
}

default_output() {
	local name=$1 os=$2 arch=$3
	printf 'dist/%s_%s_%s%s' "$name" "$os" "$arch" "$(target_suffix "$os")"
}

target_is_supported() {
	local candidate=$1 target

	while IFS= read -r target; do
		[[ $target == "$candidate" ]] && return 0
	done < <(project_targets)

	return 1
}

build_go_command() {
	local label=$1 package=$2 output=$3 cgo_enabled=$4

	info "Building optimized $TARGET $label binary: $output"
	mkdir -p "$(dirname -- "$output")"
	CGO_ENABLED=$cgo_enabled GOOS=$TARGET_OS GOARCH=$TARGET_ARCH go -C go build \
		-trimpath \
		-tags webdist \
		-ldflags "-s -w -X main.version=$VERSION" \
		-o "$output" \
		"$package"

	info "Built $output"
}

tool_cgo_enabled() {
	local label=$1 mode=$2

	if [[ $mode != cgo ]]; then
		printf '0'
		return 0
	fi

	if [[ -n ${TOOL_CGO_ENABLED:-} ]]; then
		if [[ $TOOL_CGO_ENABLED == 0 ]]; then
			printf 'Skipping %s; it requires CGO and TOOL_CGO_ENABLED=0.\n' "$label" >&2
			return 1
		fi
		printf '%s' "$TOOL_CGO_ENABLED"
		return 0
	fi

	if [[ $TARGET != "$HOST_TARGET" ]]; then
		printf 'Skipping %s for cross target %s; set TOOL_CGO_ENABLED=1 when a CGO cross toolchain is configured.\n' "$label" "$TARGET" >&2
		return 1
	fi

	if [[ $CHECK_CGO_ENABLED == 0 ]]; then
		printf 'Skipping %s; it requires CGO and go env CGO_ENABLED is 0.\n' "$label" >&2
		return 1
	fi

	printf '%s' "$CHECK_CGO_ENABLED"
}

build_web_assets() {
	info 'Building Svelte browser assets...'
	require_command npm ' to build browser assets'

	npm --prefix go/web ci
	npm --prefix go/web rebuild rolldown
	npm --prefix go/web run generate:openapi
	if [[ $RUN_TESTS == 1 && $RUN_CHECKS == 1 ]]; then
		npm --prefix go/web run check
	fi
	npm --prefix go/web run build
}

check_go_format() {
	local go_files=() unformatted

	while IFS= read -r -d '' file; do
		go_files+=("$file")
	done < <(find go -name '*.go' -not -path '*/vendor/*' -print0)

	((${#go_files[@]} > 0)) || return 0

	unformatted=$(gofmt -l "${go_files[@]}")
	[[ -z $unformatted ]] || die "gofmt required for:\n$unformatted"
}

compile_ghidra_scripts() {
	if [[ -n ${GHIDRA_INSTALL_DIR:-} ]] || command -v ghidra-analyzeHeadless >/dev/null; then
		info 'Compiling Ghidra scripts...'
		./go/tools/compile-ghidra-scripts.sh
	else
		info 'Skipping Ghidra script compile; set GHIDRA_INSTALL_DIR or add ghidra-analyzeHeadless to PATH to enable it.'
	fi
}

run_checks_and_tests() {
	if [[ $RUN_TESTS != 1 ]]; then
		return 0
	fi

	if [[ $RUN_CHECKS == 1 ]]; then
		info 'Checking formatting...'
		check_go_format

		info 'Running go vet...'
		CGO_ENABLED=$CHECK_CGO_ENABLED GOOS=$HOST_OS GOARCH=$HOST_ARCH go -C go vet ./...

		info 'Checking generated LT2 findings index...'
		CGO_ENABLED=$CHECK_CGO_ENABLED GOOS=$HOST_OS GOARCH=$HOST_ARCH go -C go run ./tools/lt2findings -check

		compile_ghidra_scripts
	fi

	info 'Running tests...'
	CGO_ENABLED=$CHECK_CGO_ENABLED GOOS=$HOST_OS GOARCH=$HOST_ARCH go -C go test ./...

	if [[ $RUN_CHECKS == 1 ]]; then
		require_command golangci-lint ' for build checks; install it or pass --skip-checks'

		info 'Running golangci-lint...'
		(cd go && CGO_ENABLED=$CHECK_CGO_ENABLED GOOS=$HOST_OS GOARCH=$HOST_ARCH golangci-lint run ./...)

		info 'Running modern Go lint checks...'
		(cd go && CGO_ENABLED=$CHECK_CGO_ENABLED GOOS=$HOST_OS GOARCH=$HOST_ARCH golangci-lint run \
			--enable=modernize \
			--enable=usestdlibvars \
			--enable=exptostd \
			--enable=prealloc \
			--enable=perfsprint \
			--enable=gocritic \
			./...)
	fi
}

build_tools() {
	local tool_dir command tool_name tool_rest tool_package tool_mode tool_cgo

	[[ $BUILD_TOOLS == 1 ]] || return 0

	tool_dir=$(dirname -- "$OUT")
	for command in "${tool_commands[@]}"; do
		tool_name=${command%%=*}
		tool_rest=${command#*=}
		tool_package=${tool_rest%=*}
		tool_mode=${tool_rest##*=}
		if tool_cgo=$(tool_cgo_enabled "$tool_name" "$tool_mode"); then
			build_go_command "$tool_name" "$tool_package" "$tool_dir/${tool_name}_${TARGET_OS}_${TARGET_ARCH}$(target_suffix "$TARGET_OS")" "$tool_cgo"
		fi
	done
}

tool_commands=(
	'rbdecompress=./tools/rbdecompress=purego'
	'lt2findings=./tools/lt2findings=cgo'
	'lt2install=./tools/lt2install=purego'
	'lt2keygen=./tools/lt2keygen=purego'
	'lt2normalize=./tools/lt2normalize=purego'
)

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
	--skip-tools)
		BUILD_TOOLS=0
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
BUILD_TOOLS=${BUILD_TOOLS:-1}
HOST_OS=$(go env GOHOSTOS)
HOST_ARCH=$(go env GOHOSTARCH)
CHECK_CGO_ENABLED=${CHECK_CGO_ENABLED:-$(go env CGO_ENABLED)}
HOST_TARGET=$(normalize_target "$HOST_OS" "$HOST_ARCH")
TARGET=$(normalize_target "$TARGET_OS" "$TARGET_ARCH")
TARGET_OS=${TARGET%/*}
TARGET_ARCH=${TARGET#*/}

target_is_supported "$TARGET" || die "unsupported target: $TARGET"

if [[ -z ${OUT:-} ]]; then
	OUT=$(default_output "$APP_NAME" "$TARGET_OS" "$TARGET_ARCH")
fi
if [[ $OUT != /* ]]; then
	OUT="$PWD/$OUT"
fi

build_web_assets
run_checks_and_tests
build_go_command "$APP_NAME" . "$OUT" 0
build_tools
