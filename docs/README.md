# Documentation

This directory centralizes project documentation that is not part of the running
website content.

## Main Docs

- `../README.md`: server quick start, deployment, API, operations, and credits.
- `tools.md`: repo-local recovery and maintenance tools.
- `reverse-engineering/overview.md`: current LT2 reverse-engineering map.
- `reverse-engineering/rb-format.md`: `Lemonade2.rb` resource container format.
- `reverse-engineering/static-unpacking.md`: current packed EXE reconstruction state.
- `reverse-engineering/normalization.md`: generated normalized-target report.
- `reverse-engineering/ghidra.md`: Ghidra build/import/annotation workflow.
- `reverse-engineering/recovered-source.md`: recovered C/H source layout.
- `reverse-engineering/teneon-exports.md`: browser DLL export map.

## Docs Kept Outside This Folder

- `README.md` stays at the repository root for GitHub and package landing pages.
- `go/web/static/project/findings/content.md` stays with web assets because the
  server embeds and renders it at runtime.
