# 🛡️ Safe Find & Replace (Go)

![Go Version](https://img.shields.io/badge/go-1.25-blue)
[![License](https://img.shields.io/badge/license-MIT-green)](./LICENSE)

**safereplace** is a safer, preview-first bulk find & replace tool for codebases. Distributed as a cross-platform, single static binary.

## ✨ Features

- **Preview First:** Dry-run by default with clear colored diffs.
- **Flexible Selection:** Select files by `--ext`, `--glob`, or `--files`.
- **Atomic Operations:** Writes are atomic and preserve file modes.
- **Safety Nets:**
  - Skips binary files.
  - Optional backups (`.bak`, `.bak.1`, ...).
  - Exit codes indicate state (`0` no changes, `1` changes/applied, `2` errors).
- **Literal Search:** Fast, exact string replacement (regex support coming soon).

## 🚀 Install

Build from source:

```bash
go build -o safereplace .
# or install directly:
go install ./...
```

## 🛠️ Usage

```bash
safereplace --pattern OLD --replace NEW [flags]
```

### Common Flags

| Flag | Description | Default |
| :--- | :--- | :--- |
| `--dry-run` | Preview changes only | `true` |
| `--no-color` | Disable colored diff output | `false` |
| `--backup` | Write `.bak` file before modifying | `false` |
| `--strict-eol` | Treat lone trailing newline changes as diffs | `false` |
| `--glob` | Glob pattern to select files | `""` |
| `--ext` | File extension to select (no dot) | `""` |
| `--files` | Comma-separated list of files | `""` |

### Examples

**Preview changes in all Go files:**
```bash
safereplace --pattern Foo --replace Bar --ext go
```

**Apply changes with backup:**
```bash
safereplace --pattern foo --replace bar --ext txt --dry-run=false --backup
```

**Target specific files using glob:**
```bash
safereplace --pattern v1 --replace v2 --glob "cmd/*/*.go"
```

## 🔍 Behavior

*   **Discovery:** Recursive search based on criteria. Ignores non-regular files and symlinks.
*   **Processor:** In-memory literal replacement.
*   **Diff:** Per-line `-`/`+` preview with `--- before`/`+++ after` headers.
*   **Apply:** Uses temp file → `fsync` → atomic rename strategy.

## 🚦 Exit Codes

*   `0`: No changes were necessary.
*   `1`: Changes were detected (in dry-run) or applied.
*   `2`: One or more errors occurred.

## 🗺️ Roadmap

- [ ] Interactive Mode (`--interactive` / `--yes`)
- [ ] Regex Mode (`--regex` + flags)
- [ ] Unified Diff Output (standard `diff` format)
- [ ] `.gitignore` Support
- [ ] Concurrency & Streaming for large codebases

## 💻 Development

```bash
# Run tests
go test ./...

# Run linter
golangci-lint run
```

> **Note:** Current release is literal-only. Regex support is planned for the next minor version.