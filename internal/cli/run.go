package cli

import (
	"errors"
	"fmt"
	"io"
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

func parseArgs(args []string) (Config, error) {
	var cfg Config
	fs := pflag.NewFlagSet("safereplace", pflag.ContinueOnError)

	fs.StringVar(&cfg.Pattern, "pattern", "", "Search pattern (required)")
	fs.StringVar(&cfg.Replace, "replace", "", "Replacement text (required)")
	fs.BoolVar(&cfg.Regex, "regex", false, "Use regex mode (default: literal)")
	fs.BoolVar(&cfg.Literal, "literal", false, "Force literal mode (default)")
	fs.StringVar(&cfg.Glob, "glob", "", "File glob to match (e.g. \"*.go\")")
	fs.StringVar(&cfg.Ext, "ext", "", "File extension filter without dot (e.g. \"txt\")")
	fs.StringSliceVar(&cfg.Files, "files", nil, "Explicit list of files")
	fs.BoolVar(&cfg.Yes, "yes", false, "Apply all changes without prompt")
	fs.BoolVar(&cfg.Interactive, "interactive", false, "Confirm per file before applying")
	fs.BoolVar(&cfg.Backup, "backup", false, "Create backups before modifying files")
	fs.BoolVar(&cfg.DryRun, "dry-run", true, "Preview changes only (default)")
	fs.BoolVar(&cfg.NoColor, "no-color", false, "Disable ANSI colors in output")
	fs.IntVar(&cfg.Context, "context", 0, "Number of context lines in diff (reserved)")
	fs.BoolVar(&cfg.StrictEOL, "strict-eol", false, "Treat a single trailing final newline difference as a change")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

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

// Run executes the CLI with the provided args and writers, returning the exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	paths, discErr := discovery.Discover(".", discovery.Selector{
		Glob:    cfg.Glob,
		Ext:     cfg.Ext,
		Files:   cfg.Files,
		Exclude: nil,
	})
	if discErr != nil && len(paths) == 0 {
		fmt.Fprintln(stderr, discErr)
		return 2
	}

	var hadErrors, hadChanges bool
	// Ensure deterministic order
	sort.Strings(paths)

	for _, p := range paths {
		res, perr := processor.SubstituteLiteralFile(p, cfg.Pattern, cfg.Replace)
		if perr != nil {
			fmt.Fprintf(stderr, "warn: %s: %v\n", p, perr)
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
			fmt.Fprintf(stderr, "warn: %s: diff error: %v\n", p, derr)
			hadErrors = true
			continue
		}
		if !changed {
			// e.g., only trailing final newline difference with StrictEOL=false
			continue
		}

		fmt.Fprintf(stdout, "file: %s  (matches: %d, replacements: %d)\n", p, res.Matches, res.Replacements)
		if cfg.DryRun {
			fmt.Fprint(stdout, preview)
		} else {
			// Apply changes safely with optional backup
			if err := apply.WriteAtomic(p, []byte(res.After), apply.Options{Backup: cfg.Backup}); err != nil {
				fmt.Fprintf(stderr, "error: apply %s: %v\n", p, err)
				hadErrors = true
				continue
			}
		}
	}

	if discErr != nil {
		fmt.Fprintln(stderr, discErr)
		hadErrors = true
	}

	if hadErrors {
		return 2
	}
	if hadChanges {
		return 1
	}
	return 0
}
