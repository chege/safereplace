package processor

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

type Result struct {
	Before       string
	After        string
	Matches      int
	Replacements int
	Changed      bool
}

// SubstituteLiteralFile reads the file, does in-memory literal replacement,
// and returns a Result. It does NOT write changes back to disk.
func SubstituteLiteralFile(path, pattern, repl string) (Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, err
	}
	// quick binary check
	if bytes.IndexByte(data, 0x00) >= 0 {
		return Result{}, fmt.Errorf("skipping binary file: %s", path)
	}

	before := string(data)
	// Empty pattern must be a no-op; otherwise ReplaceAll would inject `repl` between every rune
	if pattern == "" {
		return Result{Before: before, After: before, Matches: 0, Replacements: 0, Changed: false}, nil
	}
	matches := strings.Count(before, pattern)
	if matches == 0 {
		return Result{Before: before, After: before, Matches: 0, Replacements: 0, Changed: false}, nil
	}
	after := strings.ReplaceAll(before, pattern, repl)
	return Result{
		Before:       before,
		After:        after,
		Matches:      matches,
		Replacements: matches,
		Changed:      before != after,
	}, nil
}
