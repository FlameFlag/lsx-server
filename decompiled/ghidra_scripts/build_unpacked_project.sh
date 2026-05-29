#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
GO_DIR="${ROOT_DIR}/go"
SCRIPT_DIR="${ROOT_DIR}/decompiled/ghidra_scripts"

PACKED_PATH="${LT2_PACKED_EXE:-${ROOT_DIR}/decompiled/local/lt2_install/Lemonade2.exe}"
UNPACKED_PATH="${LT2_UNPACKED_EXE:-${ROOT_DIR}/decompiled/local/unpacked/Lemonade2.unpacked.exe}"
STATIC_MODE="${LT2_STATIC_MODE:-canonical}"
PROJECT_DIR="${LT2_GHIDRA_PROJECT_DIR:-${ROOT_DIR}/decompiled/local/ghidra_projects}"
PROJECT_NAME="${LT2_GHIDRA_PROJECT_NAME:-LT2Unpacked}"
REPORT_PATH="${LT2_NORMALIZATION_REPORT:-${ROOT_DIR}/docs/reverse-engineering/normalization.md}"
LOG_PATH="${LT2_GHIDRA_LOG:-${ROOT_DIR}/decompiled/analysis/ghidra_unpacked.log}"
ANALYSIS_TIMEOUT="${LT2_GHIDRA_TIMEOUT:-600}"

if [[ ! -f "${PACKED_PATH}" ]]; then
	printf 'missing packed Lemonade2.exe: %s\n' "${PACKED_PATH}" >&2
	printf 'install/extract it first, or set LT2_PACKED_EXE to the packed source path.\n' >&2
	exit 1
fi

mkdir -p "$(dirname "${UNPACKED_PATH}")" "${PROJECT_DIR}" "$(dirname "${LOG_PATH}")"

go -C "${GO_DIR}" run ./tools/lt2normalize \
	-packed "${PACKED_PATH}" \
	-out "${UNPACKED_PATH}" \
	-report "${REPORT_PATH}" \
	-static-mode "${STATIC_MODE}" \
	-derive-static-normalized "${UNPACKED_PATH}" \
	-check

if [[ "${LT2_SKIP_GHIDRA:-0}" == "1" ]]; then
	printf 'wrote normalized target: %s\n' "${UNPACKED_PATH}"
	printf 'skipped Ghidra import because LT2_SKIP_GHIDRA=1\n'
	exit 0
fi

GHIDRA_HEADLESS="${GHIDRA_HEADLESS:-}"
if [[ -z "${GHIDRA_HEADLESS}" ]]; then
	GHIDRA_HEADLESS="$(command -v ghidra-analyzeHeadless || true)"
fi
if [[ -z "${GHIDRA_HEADLESS}" ]]; then
	printf 'ghidra-analyzeHeadless is not on PATH; set GHIDRA_HEADLESS to its full path.\n' >&2
	exit 1
fi

"${GHIDRA_HEADLESS}" "${PROJECT_DIR}" "${PROJECT_NAME}" \
	-import "${UNPACKED_PATH}" \
	-overwrite \
	-analysisTimeoutPerFile "${ANALYSIS_TIMEOUT}" \
	-scriptPath "${SCRIPT_DIR}/src/main/java" \
	-preScript PreRepairAntiDisassembly.java \
	-postScript FullAnalysis.java \
	-log "${LOG_PATH}" \
	"$@"

printf 'wrote normalized target: %s\n' "${UNPACKED_PATH}"
printf 'updated Ghidra project: %s/%s.gpr\n' "${PROJECT_DIR}" "${PROJECT_NAME}"
