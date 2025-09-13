package main

import (
	_ "bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/pflag"
)

type Config struct {
	Pattern     string
	Replace     string
	Regex       bool
	Literal     bool
	Glob        string
	Ext         string
	Files       []string
	Yes         bool
	Interactive bool
	Backup      bool
	DryRun      bool
}

func parseArgs() (Config, error) {
	var cfg Config

	pflag.StringVar(&cfg.Pattern, "pattern", "", "Search pattern (required)")
	pflag.StringVar(&cfg.Replace, "replace", "", "Replacement text (required)")
	pflag.BoolVar(&cfg.Regex, "regex", false, "Use regex mode (default: literal)")
	pflag.BoolVar(&cfg.Literal, "literal", false, "Force literal mode (default)")
	pflag.StringVar(&cfg.Glob, "glob", "", "File glob to match (e.g. \"*.go\")")
	pflag.StringVar(&cfg.Ext, "ext", "", "File extension filter without dot (e.g. \"txt\")")
	pflag.StringSliceVar(&cfg.Files, "files", nil, "Explicit list of files")
	pflag.BoolVar(&cfg.Yes, "yes", false, "Apply all changes without prompt")
	pflag.BoolVar(&cfg.Interactive, "interactive", false, "Confirm per file before applying")
	pflag.BoolVar(&cfg.Backup, "backup", false, "Create backups before modifying files")
	pflag.BoolVar(&cfg.DryRun, "dry-run", true, "Preview changes only (default)")

	pflag.Parse()

	// Validate minimal MVP constraints
	if cfg.Pattern == "" || cfg.Replace == "" {
		return cfg, errors.New("--pattern and --replace are required")
	}
	if cfg.Regex {
		return cfg, errors.New("regex mode not yet implemented in MVP; use --literal (default)")
	}
	if cfg.Yes && cfg.Interactive {
		return cfg, errors.New("--yes and --interactive are mutually exclusive")
	}
	return cfg, nil
}

func discoverFiles(cfg Config) ([]string, error) {
	seen := map[string]struct{}{}
	add := func(path string) {
		if path == "" {
			return
		}
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			abs, _ := filepath.Abs(path)
			seen[abs] = struct{}{}
		}
	}

	// --files
	for _, f := range cfg.Files {
		add(f)
	}

	// --glob
	if cfg.Glob != "" {
		matches, _ := filepath.Glob(cfg.Glob)
		for _, m := range matches {
			add(m)
		}
	}

	// --ext (walk current directory)
	if cfg.Ext != "" {
		trimmed := strings.TrimPrefix(cfg.Ext, ".")
		_ = filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.Type().IsRegular() && strings.EqualFold(strings.TrimPrefix(filepath.Ext(path), "."), trimmed) {
				add(path)
			}
			return nil
		})
	}

	// Default: nothing if user didn’t specify any selector
	if len(seen) == 0 && cfg.Glob == "" && cfg.Ext == "" && len(cfg.Files) == 0 {
		return nil, errors.New("no files specified; use --glob, --ext, or --files")
	}

	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

type FileChange struct {
	Path         string
	BeforeCount  int
	AfterCount   int
	Replacements int
	DiffPreview  string
	Changed      bool
	Err          error
}

func processFile(path, pattern, repl string) FileChange {
	data, err := os.ReadFile(path)
	if err != nil {
		return FileChange{Path: path, Err: err}
	}

	// Quick binary guard (MVP): if NUL byte present, skip
	if bytes.IndexByte(data, 0x00) >= 0 {
		return FileChange{Path: path, Err: fmt.Errorf("skipping binary file")}
	}

	before := string(data)
	beforeCount := strings.Count(before, pattern)
	if beforeCount == 0 {
		return FileChange{Path: path, BeforeCount: 0, AfterCount: 0, Replacements: 0, Changed: false}
	}

	after := strings.ReplaceAll(before, pattern, repl)
	afterCount := strings.Count(after, pattern)
	replacements := beforeCount - afterCount

	// Simple colored diff preview (per-line). MVP – not a full unified diff.
	preview := buildLineDiffPreview(before, after)

	return FileChange{
		Path:         path,
		BeforeCount:  beforeCount,
		AfterCount:   afterCount,
		Replacements: replacements,
		DiffPreview:  preview,
		Changed:      before != after,
	}
}

func buildLineDiffPreview(before, after string) string {
	const (
		reset = "\x1b[0m"
		red   = "\x1b[31m"
		green = "\x1b[32m"
	)
	var b strings.Builder
	beforeLines := strings.Split(before, "\n")
	afterLines := strings.Split(after, "\n")

	// Naive: print both versions with +/- markers when lines differ.
	// Stops at the longer length, showing context by index.
	max := len(beforeLines)
	if len(afterLines) > max {
		max = len(afterLines)
	}
	fmt.Fprintf(&b, "--- before\n+++ after\n")
	for i := 0; i < max; i++ {
		var bl, al string
		if i < len(beforeLines) {
			bl = beforeLines[i]
		}
		if i < len(afterLines) {
			al = afterLines[i]
		}
		if bl == al {
			continue
		}
		if bl != "" {
			fmt.Fprintf(&b, "%s-%s%s\n", red, bl, reset)
		}
		if al != "" {
			fmt.Fprintf(&b, "%s+%s%s\n", green, al, reset)
		}
	}
	return b.String()
}

func main() {
	cfg, err := parseArgs()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2) // errors → exit code 2
	}

	files, err := discoverFiles(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	var (
		hadErrors  bool
		hadChanges bool
	)

	for _, f := range files {
		ch := processFile(f, cfg.Pattern, cfg.Replace)
		if ch.Err != nil {
			// Non-fatal; record and continue
			fmt.Fprintf(os.Stderr, "warn: %s: %v\n", f, ch.Err)
			hadErrors = true
			continue
		}
		if ch.Changed {
			hadChanges = true
			fmt.Printf("file: %s  (matches: %d → %d, replacements: %d)\n", ch.Path, ch.BeforeCount, ch.AfterCount, ch.Replacements)
			if cfg.DryRun {
				fmt.Print(ch.DiffPreview)
			} else {
				// Apply not implemented in MVP
				fmt.Fprintf(os.Stderr, "note: --dry-run=false apply not implemented yet; run in dry-run.\n")
				hadErrors = true
			}
		}
	}

	// Summarize
	if hadErrors {
		// If any error occurred, prefer exit code 2
		os.Exit(2)
	}
	if hadChanges {
		os.Exit(1) // changes detected (dry-run only)
	}
	os.Exit(0) // no changes
}
