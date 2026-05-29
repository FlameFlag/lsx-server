# Ghidra Workflow

The normal cleanup path is a two-stage headless run:

1. `PreRepairAntiDisassembly.java` runs as a `-preScript` before auto-analysis.
2. `FullAnalysis.java` runs as a `-postScript` after analysis creates the normal
   function and type surface.

`PopulateFindings.java` reads `decompiled/findings/findings.ini`, applies labels,
plate comments, bookmarks, source-map entries, and `LT2_*` function tags.

## Build Scripts

Compile the Java scripts before long Ghidra runs:

```sh
./tools/compile-ghidra-scripts.sh
```

The script entrypoints stay in the default package so headless commands such as
`-postScript FullAnalysis.java` keep working. Shared helpers live under
`com.lemonadetycoon.ghidra`.

## Build The Current Game-Code Project

```sh
decompiled/ghidra_scripts/build_unpacked_project.sh
```

That wrapper derives `decompiled/local/unpacked/Lemonade2.unpacked.exe`, validates
it, imports it as `LT2Unpacked`, and runs the pre/post scripts. Set
`LT2_STATIC_MODE=portable` for the semantically cleaner packed-file-derived
output instead of the canonical historical dump-artifact-compatible output.

Manual equivalent:

```sh
go -C go run ./tools/lt2normalize \
  -derive-static-normalized decompiled/local/unpacked/Lemonade2.unpacked.exe \
  -check

ghidra-analyzeHeadless decompiled/local/ghidra_projects LT2Unpacked \
  -import decompiled/local/unpacked/Lemonade2.unpacked.exe \
  -overwrite \
  -analysisTimeoutPerFile 600 \
  -scriptPath decompiled/ghidra_scripts/src/main/java \
  -preScript PreRepairAntiDisassembly.java \
  -postScript FullAnalysis.java \
  -log decompiled/analysis/ghidra_unpacked.log
```

## Refresh Findings

```sh
go -C go run ./tools/lt2findings -write
go -C go run ./tools/lt2findings -check
```

`lt2findings` regenerates `decompiled/src/generated/findings_index.h` with
`LT2_FINDING(...)`, `LT2_C_SYMBOL(...)`, `LT2_FINDING_TAG(...)`, and
`LT2_C_SYMBOL_TAG(...)` streams.

## Target Split

- `Lemonade2.exe`: protected-loader entry, virtual original sections, protector
  strings, and runtime-dump warning.
- `Lemonade2.unpacked.exe`: normalized unpacked game-code markers and concrete
  LSX functions.
- `TeneonIERelease.dll`: browser wrapper exports and `LoadURL` GET-navigation
  boundary.
- `fmod.dll`: third-party audio dependency marker.
- `Lemonade Tycoon 2 - New York City.exe`: installer-side license manager.
- `pdata_002_0x04D15C_pe32_dll.dll`: extracted Armadillo runtime DLL.
