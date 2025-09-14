package diff

import (
	"strings"
)

// Options control how the diff output is rendered.
// Context is reserved for future unified-diff support; currently ignored (0).
// If Color is true, added/removed lines are wrapped with ANSI colors.
//
// This is a minimal, dependency-free implementation suitable for MVP.
// We can later switch internals to github.com/pmezard/go-difflib while
// keeping this public API stable.
type Options struct {
	Color   bool
	Context int
	// StrictEOL controls whether differences in a single trailing final newline are treated as changes.
	// When true, a difference in a lone trailing newline is reported as a change. When false (default), such differences are ignored.
	StrictEOL bool
}

// HasChanges reports whether the inputs differ.
func HasChanges(before, after string) bool { return before != after }

// equalIgnoringSingleTrailingFinalNL returns true if a and b are equal, or if they differ only by a single trailing final newline.
func equalIgnoringSingleTrailingFinalNL(a, b string) bool {
	if a == b {
		return true
	}
	// If exactly one side ends with a single trailing \n, consider equal when contents match without that \n.
	if strings.HasSuffix(a, "\n") && !strings.HasSuffix(b, "\n") {
		return strings.TrimSuffix(a, "\n") == b
	}
	if strings.HasSuffix(b, "\n") && !strings.HasSuffix(a, "\n") {
		return strings.TrimSuffix(b, "\n") == a
	}
	return false
}

// Diff returns a human-readable diff preview and whether there were changes.
// For MVP it emits a simple per-line `---/+++` header and `-`/`+` lines when
// corresponding lines differ. Identical lines are elided. Context is ignored.
func Diff(before, after string, opts Options) (string, bool, error) {
	// Ignore a lone trailing final newline difference by default (unless StrictEOL)
	if !opts.StrictEOL && equalIgnoringSingleTrailingFinalNL(before, after) {
		return "", false, nil
	}
	changed := before != after
	if !changed {
		return "", false, nil
	}

	const (
		ansiReset = "\x1b[0m"
		ansiRed   = "\x1b[31m"
		ansiGreen = "\x1b[32m"
	)

	colorize := func(prefix, line string, added bool) string {
		if !opts.Color {
			return prefix + line
		}
		if added {
			return ansiGreen + prefix + line + ansiReset
		}
		return ansiRed + prefix + line + ansiReset
	}

	var b strings.Builder
	b.WriteString("--- before\n")
	b.WriteString("+++ after\n")

	bl := strings.Split(before, "\n")
	al := strings.Split(after, "\n")
	max := len(bl)
	if len(al) > max {
		max = len(al)
	}
	for i := 0; i < max; i++ {
		var br, ar string
		if i < len(bl) {
			br = bl[i]
		}
		if i < len(al) {
			ar = al[i]
		}
		if br == ar {
			continue
		}
		if br != "" {
			b.WriteString(colorize("-", br, false))
			b.WriteByte('\n')
		}
		if ar != "" {
			b.WriteString(colorize("+", ar, true))
			b.WriteByte('\n')
		}
	}
	return b.String(), true, nil
}
