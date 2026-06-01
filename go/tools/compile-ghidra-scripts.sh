#!/usr/bin/env nix-shell
#! nix-shell -i bash -p bash coreutils findutils jdk21_headless
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT_DIR="${ROOT_DIR}/decompiled/ghidra_scripts"

if [[ -z "${GHIDRA_INSTALL_DIR:-}" ]]; then
	if command -v ghidra-analyzeHeadless >/dev/null 2>&1; then
		HEADLESS="$(readlink -f "$(command -v ghidra-analyzeHeadless)" 2>/dev/null || command -v ghidra-analyzeHeadless)"
		GHIDRA_INSTALL_DIR="$(cd "$(dirname "${HEADLESS}")/.." && pwd)"
	else
		echo "GHIDRA_INSTALL_DIR is not set and ghidra-analyzeHeadless is not on PATH" >&2
		exit 1
	fi
fi

JAVA_HOME_CANDIDATES=(
	"${JAVA_HOME:-}"
)

JAVAC=""
for candidate in "${JAVA_HOME_CANDIDATES[@]}"; do
	if [[ -n "${candidate}" && -x "${candidate}/bin/javac" ]]; then
		JAVAC="${candidate}/bin/javac"
		break
	fi
done

if [[ -z "${JAVAC}" ]]; then
	JAVAC="$(command -v javac || true)"
fi

if [[ -z "${JAVAC}" ]]; then
	echo "Could not locate javac. Set JAVA_HOME to a JDK 21 install." >&2
	exit 1
fi

CLASSPATH="$(
	find "${GHIDRA_INSTALL_DIR}/Ghidra/Framework" \
		"${GHIDRA_INSTALL_DIR}/Ghidra/Features" \
		"${GHIDRA_INSTALL_DIR}/Ghidra/Debug" \
		"${GHIDRA_INSTALL_DIR}/Ghidra/Processors" \
		"${GHIDRA_INSTALL_DIR}/support" \
		-type f -name '*.jar' -print | paste -sd ':' -
)"

BUILD_DIR="${SCRIPT_DIR}/build/classes"
rm -rf "${BUILD_DIR}"
mkdir -p "${BUILD_DIR}"

mapfile -t SOURCES < <(find "${SCRIPT_DIR}/src/main/java" -type f -name '*.java' -print | sort)

"${JAVAC}" -encoding UTF-8 --release 21 -proc:none \
	-Xlint:deprecation -Xlint:unchecked \
	-cp "${CLASSPATH}" \
	-d "${BUILD_DIR}" \
	"${SOURCES[@]}"

echo "Compiled $(find "${BUILD_DIR}" -name '*.class' | wc -l | tr -d ' ') class files into ${BUILD_DIR}"
