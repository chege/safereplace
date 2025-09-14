package discovery

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Selector defines how files are selected for processing.
// Provide at least one of Glob, Ext, or Files.
// Ext should be provided without a leading dot (e.g., "txt").
type Selector struct {
	Glob    string
	Ext     string
	Files   []string
	Exclude []string
}

// Discover returns absolute, deduplicated, sorted paths to regular files under root
// according to the selector. It performs no I/O beyond file system queries,
// prints nothing, and is deterministic in its output ordering.
func Discover(root string, sel Selector) ([]string, error) {
	normRoot, normSel, err := normalize(root, sel)
	if err != nil {
		return nil, err
	}

	if normSel.Glob == "" && normSel.Ext == "" && len(normSel.Files) == 0 {
		return nil, errors.New("discovery: at least one of --glob, --ext or --files must be provided")
	}

	resultSet := make(map[string]struct{})
	var errs []error

	// Expand explicit files first
	if len(normSel.Files) > 0 {
		paths, ferrs := expandFiles(normRoot, normSel.Files)
		for _, p := range paths {
			resultSet[p] = struct{}{}
		}
		errs = append(errs, ferrs...)
	}

	// Expand glob
	if normSel.Glob != "" {
		paths, gerrs := expandGlob(normRoot, normSel.Glob)
		for _, p := range paths {
			resultSet[p] = struct{}{}
		}
		errs = append(errs, gerrs...)
	}

	// Expand by extension walk
	if normSel.Ext != "" {
		paths, werrs := expandExt(normRoot, normSel.Ext)
		for _, p := range paths {
			resultSet[p] = struct{}{}
		}
		errs = append(errs, werrs...)
	}

	// To slice
	paths := make([]string, 0, len(resultSet))
	for p := range resultSet {
		paths = append(paths, p)
	}

	// Apply excludes (if any)
	if len(normSel.Exclude) > 0 && len(paths) > 0 {
		paths = applyExcludes(normRoot, paths, normSel.Exclude)
	}

	// Deterministic order
	sort.Strings(paths)

	// Join errors (non-fatal during discovery)
	if len(errs) > 0 {
		return paths, errors.Join(errs...)
	}
	return paths, nil
}

// --- internals ---

func normalize(root string, sel Selector) (string, Selector, error) {
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", sel, fmt.Errorf("discovery: invalid root: %w", err)
	}

	sel.Ext = strings.TrimPrefix(sel.Ext, ".")
	// Leave Glob and Exclude as-is (may be relative to root)
	return absRoot, sel, nil
}

func isRegular(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	// Skip symlinks & non-regular files
	if info.Mode()&fs.ModeSymlink != 0 {
		return false
	}
	return info.Mode().IsRegular()
}

func toAbsUnderRoot(root, p string) (string, error) {
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func expandFiles(root string, files []string) ([]string, []error) {
	var out []string
	var errs []error
	for _, f := range files {
		abs, err := toAbsUnderRoot(root, f)
		if err != nil {
			errs = append(errs, fmt.Errorf("files: %s: %w", f, err))
			continue
		}
		if isRegular(abs) {
			out = append(out, abs)
		}
	}
	return out, errs
}

func expandGlob(root, pattern string) ([]string, []error) {
	var errs []error
	// If the pattern is not absolute, make it relative to root.
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(root, pattern)
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		// Bad pattern is a hard error for this branch, but we keep going elsewhere.
		errs = append(errs, fmt.Errorf("glob: %w", err))
		return nil, errs
	}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if isRegular(m) {
			abs, _ := filepath.Abs(m) // Abs should succeed for Glob results
			out = append(out, abs)
		}
	}
	return out, errs
}

func expandExt(root, ext string) ([]string, []error) {
	var out []string
	var errs []error
	target := strings.ToLower(strings.TrimPrefix(ext, "."))
	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Collect and continue
			errs = append(errs, fmt.Errorf("walk: %s: %w", path, err))
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if strings.EqualFold(strings.TrimPrefix(filepath.Ext(path), "."), target) {
			abs, aerr := filepath.Abs(path)
			if aerr != nil {
				errs = append(errs, fmt.Errorf("abs: %s: %w", path, aerr))
				return nil
			}
			out = append(out, abs)
		}
		return nil
	}
	_ = filepath.WalkDir(root, walkFn)
	return out, errs
}

func applyExcludes(root string, paths []string, excludes []string) []string {
	filtered := make([]string, 0, len(paths))
nextPath:
	for _, p := range paths {
		rel, _ := filepath.Rel(root, p)
		base := filepath.Base(p)
		for _, ex := range excludes {
			// Try match against relative path and base name.
			if match(ex, rel) || match(ex, base) {
				continue nextPath
			}
			// If exclude is absolute, try match on absolute path as well.
			if filepath.IsAbs(ex) && match(ex, p) {
				continue nextPath
			}
		}
		filtered = append(filtered, p)
	}
	return filtered
}

// filepath.Match returns error for malformed patterns; treat that as non-match here.
func match(pattern, name string) bool {
	ok, err := filepath.Match(pattern, name)
	return err == nil && ok
}
