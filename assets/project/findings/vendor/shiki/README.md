This directory is populated by `go generate ./assets`.

Generated Shiki browser modules are intentionally ignored by git. If they are
present when `go build` runs, Go embeds them into the binary.
