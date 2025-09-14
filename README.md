# Safe Find & Replace (Go)

Safer, preview-first bulk find & replace for codebases. Cross‑platform, single static binary.

## Features
- Dry-run by default with clear diffs (ANSI color optional).
- Literal search (regex later).
- Select files by `--ext`, `--glob`, or `--files`.
- Optional backups (`.bak`, `.bak.1`, …).
- Exit codes: `0` no changes, `1` changes/applied, `2` errors.
- Skips binaries, preserves existing EOLs, atomic writes.

## Install
```bash
go build -o safereplace .
# or: go install ./...
```

## Usage
```bash
safereplace --pattern OLD --replace NEW [--ext txt | --glob "src/**/*.go" | --files a.go,b.go]
```

Common flags:
- `--dry-run` (default true) — preview only.
- `--no-color` — disable colored diff.
- `--context N` — reserved (unified context later).
- `--strict-eol` — treat lone trailing newline changes as diffs.
- `--backup` — write `.bak` before modifying.
- `--glob` / `--ext` / `--files` — choose files to process.

### Examples
Dry-run, all `.go` files:
```bash
safereplace --pattern Foo --replace Bar --ext go --no-color
```
Apply with backup:
```bash
safereplace --pattern foo --replace bar --ext txt --dry-run=false --backup
```
Glob:
```bash
safereplace --pattern v1 --replace v2 --glob "cmd/*/*.go"
```
Explicit files:
```bash
safereplace --pattern TODO --replace DONE --files README.md,docs/notes.md
```

## Behavior
- **Discovery:** recursive by `--ext`, `--glob`, or `--files`. Non-regular & symlinks ignored.
- **Processor:** in-memory literal replace; counts matches.
- **Diff:** simple per-line `-`/`+` preview; headers `--- before`/`+++ after`.
- **Apply:** same-dir temp file → fsync → atomic rename; optional backup; preserves file mode.

## Exit Codes
- `0` — no changes.
- `1` — changes detected/applied.
- `2` — one or more errors.

## Roadmap
- Interactive/`--yes`
- Regex mode + options
- Real unified diffs (`go-difflib`)
- Excludes / `.gitignore` support
- Concurrency & streaming

## Development
```bash
go test ./...
# if using golangci-lint
golangci-lint run
```

> Note: current release is literal-only; regex to follow.
