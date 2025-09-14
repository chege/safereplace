package main

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/pflag"

	"safereplace/internal/apply"
	"safereplace/internal/diff"
	"safereplace/internal/discovery"
	"safereplace/internal/processor"
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
	NoColor     bool
	Context     int
	StrictEOL   bool
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
	pflag.BoolVar(&cfg.NoColor, "no-color", false, "Disable ANSI colors in output")
	pflag.IntVar(&cfg.Context, "context", 0, "Number of context lines in diff (reserved)")
	pflag.BoolVar(&cfg.StrictEOL, "strict-eol", false, "Treat a single trailing final newline difference as a change")

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
	if cfg.Glob == "" && cfg.Ext == "" && len(cfg.Files) == 0 {
		return cfg, errors.New("no files specified; use --glob, --ext, or --files")
	}
	return cfg, nil
}

func main() {
	cfg, err := parseArgs()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	paths, discErr := discovery.Discover(".", discovery.Selector{
		Glob:    cfg.Glob,
		Ext:     cfg.Ext,
		Files:   cfg.Files,
		Exclude: nil,
	})
	if discErr != nil && len(paths) == 0 {
		fmt.Fprintln(os.Stderr, discErr)
		os.Exit(2)
	}

	var hadErrors, hadChanges bool
	// Ensure deterministic order
	sort.Strings(paths)

	for _, p := range paths {
		res, perr := processor.SubstituteLiteralFile(p, cfg.Pattern, cfg.Replace)
		if perr != nil {
			fmt.Fprintf(os.Stderr, "warn: %s: %v\n", p, perr)
			hadErrors = true
			continue
		}
		if !res.Changed {
			continue
		}
		hadChanges = true

		opts := diff.Options{Color: !cfg.NoColor, Context: cfg.Context, StrictEOL: cfg.StrictEOL}
		preview, changed, derr := diff.Diff(res.Before, res.After, opts)
		if derr != nil {
			fmt.Fprintf(os.Stderr, "warn: %s: diff error: %v\n", p, derr)
			hadErrors = true
			continue
		}
		if !changed {
			// e.g., only trailing final newline difference with StrictEOL=false
			continue
		}

		fmt.Printf("file: %s  (matches: %d, replacements: %d)\n", p, res.Matches, res.Replacements)
		if cfg.DryRun {
			fmt.Print(preview)
		} else {
			// Apply changes safely with optional backup
			if err := apply.WriteAtomic(p, []byte(res.After), apply.Options{Backup: cfg.Backup}); err != nil {
				fmt.Fprintf(os.Stderr, "error: apply %s: %v\n", p, err)
				hadErrors = true
				continue
			}
		}
	}

	if discErr != nil {
		fmt.Fprintln(os.Stderr, discErr)
		hadErrors = true
	}

	if hadErrors {
		os.Exit(2)
	}
	if hadChanges {
		os.Exit(1)
	}
	os.Exit(0)
}
