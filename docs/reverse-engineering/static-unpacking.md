# Static Unpacking

The protected `Lemonade2.exe` matches an Armadillo-style protected executable.
The current workflow can derive normalized game-code targets without a live
Windows process.

## Current Status

- Parent-side bulk writes are ruled out as the final payload source.
- `.pdata`, `.adata`, and `.text1` loader stages have static reconstruction paths.
- The static pipeline derives the original `.text`, `.rdata`, and `.data` payload
  from packed mapper-stream bytes.
- `lt2normalize` can produce canonical, portable, and strict normalized outputs.
- Canonical mode reproduces the historical dump-artifact-compatible normalized
  hash; strict mode is the packed-file-only semantic output.

## Normal Output

```sh
go -C go run ./tools/lt2normalize \
  -derive-static-normalized decompiled/local/unpacked/Lemonade2.unpacked.exe \
  -check
```

Optional modes:

```sh
go -C go run ./tools/lt2normalize \
  -static-mode portable \
  -derive-static-normalized decompiled/local/unpacked/Lemonade2.portable.exe

go -C go run ./tools/lt2normalize \
  -static-mode strict \
  -derive-clean-normalized decompiled/local/unpacked/Lemonade2.clean-candidate.exe
```

## Known Hashes

| Artifact | SHA-256 |
| --- | --- |
| Canonical payload | `0a14f853214920d91abbb596a369efbb2a3a6ff5bc9e93e8c41500aa5c0d1f7f` |
| Canonical normalized EXE | `ce1ff9868c70e49ce7123684341a49b10627e731d769daa4b568421726ea4caa` |
| Portable payload | `babd032c53ceb22909155ef5a7e602ae40bd85c76ea7119c022729004acddb64` |
| Portable normalized EXE | `8db2008a47aa4fa1c865a82a2e225a88a8df67d626af924ebf18d45c7f9ca11f` |
| Strict payload | `98426f442bd67fc6fadeabf6bf2bae562de5dea4effa52abb4ab7373b56fd7ba` |
| Strict normalized EXE | `34247cbbec6fcf04933d4653454892bc66570f94f8128e577369367342ba028c` |

## Static-Unpacking Workspace

`decompiled/analysis/lemonade2_static_unpacking/` remains the code workspace for
instrumentation, inspectors, traces, and reconstruction helpers. Its Markdown
notes were removed because the important current state is captured here and in
`docs/tools.md`.

Useful commands:

```sh
make -C decompiled/analysis/lemonade2_static_unpacking check
make -C decompiled/analysis/lemonade2_static_unpacking ghidra
```

Do not promote new reconstruction logic into `lt2normalize` until the static
output matches known bytes or hashes byte-for-byte.
