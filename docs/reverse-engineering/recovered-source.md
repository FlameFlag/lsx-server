# Recovered Source Layout

`decompiled/src/` holds the human-readable recovered C/H artifacts. Address-backed
findings live in `decompiled/findings/findings.ini`; generated indexes live in
`decompiled/src/generated/findings_index.h`.

```text
browser/      TeneonIERelease.dll browser wrapper recovery
common/       lightweight Win32 and recovered support types
formats/      Lemonade2.rb container/data-format reconstruction
generated/    X-macro indexes generated from findings.ini
installer/    installer-side license manager and registration code
network/      recovered LSX client/server protocol compatibility notes
protection/   Lemonade2.exe protector/packed-loader analysis
```

## Trust Levels

- `network/lsx_client_protocol.*`: refreshed against `Lemonade2.unpacked.exe`.
  Key functions include score upload `0x004073C0`, account creation `0x0045FB70`,
  checksum `0x00410030`, packed date scalar `0x00418FF0`, and local browser path
  builder `0x00420F10`.
- `browser/`, `installer/`, and `formats/`: separate DLL, installer, or data-format
  recoveries; not invalidated by the game executable unpacking changes.
- `protection/lemonade2_protected_entry.*`: packed entry/protector evidence only.
- `protection/lemonade2_armadillo_protector.*`: historical protector sketch; keep
  it as context, not authoritative loader pseudocode.

Regenerate indexes with:

```sh
go -C go run ./tools/lt2findings -write
```
