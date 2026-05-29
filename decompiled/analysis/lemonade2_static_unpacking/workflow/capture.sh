#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG="$ROOT/config/local.env"
BUILD_ONLY_FLAG=""
for arg in "$@"; do
  case "$arg" in
    --build-only)
      BUILD_ONLY_FLAG="-BuildOnly"
      ;;
    *)
      CONFIG="$arg"
      ;;
  esac
done

if [[ -f "$CONFIG" ]]; then
  # shellcheck disable=SC1090
  source "$CONFIG"
elif [[ -z "${LEMONADE2_VM_SSH:-}" || -z "${LEMONADE2_REMOTE_ROOT:-}" || -z "${LEMONADE2_GAME_EXE:-}" ]]; then
  printf 'Missing config: %s\nCopy config/example.env to config/local.env or export LEMONADE2_* settings.\n' "$CONFIG" >&2
  exit 1
fi

: "${LEMONADE2_VM_SSH:?LEMONADE2_VM_SSH is required}"
: "${LEMONADE2_REMOTE_ROOT:?LEMONADE2_REMOTE_ROOT is required}"
: "${LEMONADE2_GAME_EXE:?LEMONADE2_GAME_EXE is required}"
LEMONADE2_RUN_SECONDS="${LEMONADE2_RUN_SECONDS:-60}"
LEMONADE2_DUMP_MEMORY="${LEMONADE2_DUMP_MEMORY:-1}"
LEMONADE2_GAME_ARGS="${LEMONADE2_GAME_ARGS:-}"
LEMONADE2_AUTO_REGISTER="${LEMONADE2_AUTO_REGISTER:-0}"
LEMONADE2_DISABLE_SEED_HOOK="${LEMONADE2_DISABLE_SEED_HOOK:-0}"

RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)"
RUN_DIR="$ROOT/runs/$RUN_ID"
mkdir -p "$RUN_DIR"
if [[ "${LEMONADE2_BUILD_ONLY:-0}" != "0" ]]; then
  BUILD_ONLY_FLAG="-BuildOnly"
fi

SSH=(ssh)
SCP=(scp)
if [[ -n "${LEMONADE2_VM_PASSWORD:-}" ]] && command -v sshpass >/dev/null 2>&1; then
  SSH=(sshpass -p "$LEMONADE2_VM_PASSWORD" ssh -o PreferredAuthentications=password -o PubkeyAuthentication=no)
  SCP=(sshpass -p "$LEMONADE2_VM_PASSWORD" scp -o PreferredAuthentications=password -o PubkeyAuthentication=no)
fi

remote_ps_path() {
  printf '%s' "$1" | sed 's#/#\\#g'
}

REMOTE_ROOT_PS="$(remote_ps_path "$LEMONADE2_REMOTE_ROOT")"
"${SSH[@]}" "$LEMONADE2_VM_SSH" "powershell -NoProfile -ExecutionPolicy Bypass -Command \"New-Item -ItemType Directory -Force -Path '$REMOTE_ROOT_PS' | Out-Null\""

"${SCP[@]}" \
  "$ROOT/instrumentation/api/hook.c" \
  "$ROOT/instrumentation/api/launcher.c" \
  "$ROOT/workflow/remote.ps1" \
  "$LEMONADE2_VM_SSH:$LEMONADE2_REMOTE_ROOT/"

DUMP_FLAG=""
if [[ "$LEMONADE2_DUMP_MEMORY" != "0" ]]; then
  DUMP_FLAG="-DumpMemory"
fi
DATA_GUARD_FLAG=""
if [[ "${LEMONADE2_DATA_GUARD:-0}" != "0" ]]; then
  DATA_GUARD_FLAG="-DataGuard"
fi
AUTO_REGISTER_FLAG=""
if [[ "$LEMONADE2_AUTO_REGISTER" != "0" ]]; then
  AUTO_REGISTER_FLAG="-AutoRegister"
fi
DISABLE_SEED_HOOK_FLAG=""
if [[ "$LEMONADE2_DISABLE_SEED_HOOK" != "0" ]]; then
  DISABLE_SEED_HOOK_FLAG="-DisableSeedHook"
fi

"${SSH[@]}" "$LEMONADE2_VM_SSH" "powershell -NoProfile -ExecutionPolicy Bypass -Command \"& '$REMOTE_ROOT_PS\\remote.ps1' -WorkDir '$REMOTE_ROOT_PS' -GameExe '$LEMONADE2_GAME_EXE' -GameArgs '$LEMONADE2_GAME_ARGS' -RunSeconds $LEMONADE2_RUN_SECONDS $DUMP_FLAG $DATA_GUARD_FLAG $AUTO_REGISTER_FLAG $DISABLE_SEED_HOOK_FLAG $BUILD_ONLY_FLAG\"" | tee "$RUN_DIR/remote.log"

"${SCP[@]}" "$LEMONADE2_VM_SSH:$LEMONADE2_REMOTE_ROOT/capture.zip" "$RUN_DIR/capture.zip"
if command -v uv >/dev/null 2>&1; then
  (cd "$ROOT" && uv run python -m workflow.report "$RUN_DIR/capture.zip" --output "$RUN_DIR/report.md")
  (cd "$ROOT" && uv run python -m inspectors.payload.snapshots "$RUN_DIR" --output "$RUN_DIR/payload_snapshot_analysis.md")
  (cd "$ROOT" && uv run python -m inspectors.data.guards "$RUN_DIR" --output "$RUN_DIR/data_guard_analysis.md")
  (cd "$ROOT" && uv run python -m reconstruction.payload.candidate "$RUN_DIR" --output-dir "$RUN_DIR/reconstruction")
  (cd "$ROOT" && uv run python -m inspectors.data.semantics "$RUN_DIR/reconstruction" --output "$RUN_DIR/reconstruction/data_patch_semantics.md")
  for generated_pe in "$RUN_DIR"/generated_island_*.bin; do
    [[ -f "$generated_pe" ]] || continue
    (cd "$ROOT" && uv run python -m inspectors.generated.pe "$generated_pe" --output "${generated_pe%.bin}_analysis.md")
  done
  if compgen -G "$RUN_DIR/generated_island_state_*.bin" >/dev/null; then
    (cd "$ROOT" && uv run python -m inspectors.generated.timeline "$RUN_DIR" --output "$RUN_DIR/generated_timeline.md")
    (cd "$ROOT" && uv run python -m reconstruction.generated.teas "$RUN_DIR" --output "$RUN_DIR/generated_tea_replay.md")
  fi
else
  (cd "$ROOT" && python3 -m workflow.report "$RUN_DIR/capture.zip" --output "$RUN_DIR/report.md")
  (cd "$ROOT" && python3 -m inspectors.payload.snapshots "$RUN_DIR" --output "$RUN_DIR/payload_snapshot_analysis.md")
  (cd "$ROOT" && python3 -m inspectors.data.guards "$RUN_DIR" --output "$RUN_DIR/data_guard_analysis.md")
  (cd "$ROOT" && python3 -m reconstruction.payload.candidate "$RUN_DIR" --output-dir "$RUN_DIR/reconstruction")
  (cd "$ROOT" && python3 -m inspectors.data.semantics "$RUN_DIR/reconstruction" --output "$RUN_DIR/reconstruction/data_patch_semantics.md")
  for generated_pe in "$RUN_DIR"/generated_island_*.bin; do
    [[ -f "$generated_pe" ]] || continue
    (cd "$ROOT" && python3 -m inspectors.generated.pe "$generated_pe" --output "${generated_pe%.bin}_analysis.md")
  done
  if compgen -G "$RUN_DIR/generated_island_state_*.bin" >/dev/null; then
    (cd "$ROOT" && python3 -m inspectors.generated.timeline "$RUN_DIR" --output "$RUN_DIR/generated_timeline.md")
    (cd "$ROOT" && python3 -m reconstruction.generated.teas "$RUN_DIR" --output "$RUN_DIR/generated_tea_replay.md")
  fi
fi
ln -sfn "$RUN_DIR" "$ROOT/runs/latest"

printf 'Run directory: %s\nReport: %s\n' "$RUN_DIR" "$RUN_DIR/report.md"
if command -v uv >/dev/null 2>&1; then
  (cd "$ROOT" && uv run python - "$RUN_DIR/report.md" <<'PY'
from pathlib import Path
import sys

text = Path(sys.argv[1]).read_text(encoding="utf-8", errors="replace").splitlines()
for line in text[:35]:
    print(line)
PY
  )
fi
