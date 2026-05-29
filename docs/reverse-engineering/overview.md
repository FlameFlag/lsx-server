# Reverse Engineering Overview

This folder is the central home for Lemonade Tycoon 2 reverse-engineering notes.
The operational source of truth remains `decompiled/findings/findings.ini`; these
docs explain the current state and how to rebuild the evidence.

## Current State

| Area | Status | Current reference |
| --- | --- | --- |
| Installed payload extraction | Reproducible | `docs/tools.md`, `lt2install` |
| RB resource bundle | Structurally mapped | `docs/reverse-engineering/rb-format.md` |
| Browser wrapper DLL | Demangled and summarized | `docs/reverse-engineering/teneon-exports.md` |
| Packed game executable | Armadillo-style protector identified | `docs/reverse-engineering/static-unpacking.md` |
| Normalized game target | Statically derivable | `docs/reverse-engineering/normalization.md` |
| Ghidra annotations | Scripted from findings INI | `docs/reverse-engineering/ghidra.md` |
| Recovered C/H artifacts | Organized by domain | `docs/reverse-engineering/recovered-source.md` |
| Public LSX/server findings | Published in app content | `go/web/static/project/findings/content.md` |

## Important Paths

- `decompiled/findings/findings.ini`: address-backed annotation source of truth.
- `decompiled/src/`: human-readable recovered C/H artifacts.
- `decompiled/ghidra_scripts/`: Ghidra headless scripts and Java helpers.
- `decompiled/local/`: ignored local binaries, extracted files, and Ghidra projects.
- `go/tools/`: reproducible recovery and validation tools.

## Trust Rules

- Use `decompiled/local/unpacked/Lemonade2.unpacked.exe` for game-code recovery.
- Treat packed `decompiled/local/lt2_install/Lemonade2.exe` as protector evidence,
  not direct game logic.
- Keep new address-backed claims in `decompiled/findings/findings.ini`, then run
  `go -C go run ./tools/lt2findings -write` and `go -C go run ./tools/lt2findings -check`.
- Keep the embedded website findings file in place; it is loaded at runtime by the
  Go web server.

## Removed Noise

The previous scattered Markdown set included stale session ledgers, historical
cleanup audits, and duplicated static-unpacking TODOs. Those were folded into the
current docs here or removed when they described superseded workflow state.
