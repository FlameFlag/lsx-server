# Tool Reference

Repo-local tools live under `go/tools/`. Run Go tools from the `go/` directory or
with `go -C go run ./tools/<name>` from the repository root.

## `lt2install`

Recreates the Lemonade Tycoon 2 install directory from the Clickteam setup EXE
without running the Windows installer.

```sh
go -C go run ./tools/lt2install -- \
  "decompiled/local/installers/Lemonade Tycoon 2 - New York City.exe" \
  decompiled/local/lt2_install
```

Use `-force` to overwrite an existing install directory. Extraction is
manifest-driven and verifies carved payloads by size and MD5.

## `lt2normalize`

Inspects the focused `Lemonade2.exe`, extracts protector assets, derives mapper
stages, and writes normalized unpacked targets for Ghidra.

```sh
go -C go run ./tools/lt2normalize -extract-pdata decompiled/local/pdata_assets
go -C go run ./tools/lt2normalize -derive-static-payload decompiled/local/static_payload.bin
go -C go run ./tools/lt2normalize -derive-static-normalized decompiled/local/unpacked/Lemonade2.unpacked.exe
go -C go run ./tools/lt2normalize -static-mode portable -derive-static-normalized decompiled/local/unpacked/Lemonade2.portable.exe
go -C go run ./tools/lt2normalize -static-mode strict -derive-clean-normalized decompiled/local/unpacked/Lemonade2.clean-candidate.exe
go -C go run ./tools/lt2normalize -check
```

See `docs/reverse-engineering/static-unpacking.md` and
`docs/reverse-engineering/normalization.md` for current hashes and status.

## `lt2findings`

Synchronizes reverse-engineering annotations between `findings.ini`, Ghidra, and
the generated recovered-source index.

```sh
go -C go run ./tools/lt2findings -write
go -C go run ./tools/lt2findings -check
```

The source of truth is `decompiled/findings/findings.ini`.

## `lt2rb`

Extracts `Lemonade2.rb`, exports image records, and validates RB round-tripping.

```sh
go -C go run ./tools/lt2rb -- "/path/to/Lemonade Tycoon 2 - New York City.exe"
go -C go run ./tools/lt2rb -extract-images ./lt2-images -- "Lemonade Tycoon 2 - New York City.exe"
go -C go run ./tools/lt2rb -rb-input -extract-images ./lt2-images Lemonade2.rb
go -C go run ./tools/lt2rb -roundtrip-md5 -- "Lemonade Tycoon 2 - New York City.exe"
```

A C23 implementation remains under `go/tools/lt2rb/c/` and builds with
`./build.sh` from that directory.

## `shortv3derive`

Documents and verifies the Lemonade2 Armadillo ShortV3 signed-key parameters used
by the keygen work.

```sh
nix develop -c make recover-shortv3
go -C go run ./tools/shortv3derive
```

Expected recovered exponent:

```text
0x70301169DE7C75D66F
```

## `keyvis_search`

Standalone C++23 project for searching Armadillo ShortV3 keygen outputs for
visually low activation keys.

```sh
make -C go/tools/keyvis_search
go/tools/keyvis_search/keyvis-search --help
```
