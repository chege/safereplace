package discovery

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"
)

func writeFile(t *testing.T, dir, rel, content string) string {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	return abs
}

func TestDiscover_ErrOnEmptySelector(t *testing.T) {
	root := t.TempDir()
	_, err := Discover(root, Selector{})
	if err == nil {
		t.Fatalf("expected error when no selectors provided")
	}
}

func TestDiscover_ByExtRecursive(t *testing.T) {
	root := t.TempDir()
	a := writeFile(t, root, "a.txt", "x")
	b := writeFile(t, root, "b.TXT", "y") // case-insensitive
	_ = writeFile(t, root, "c.md", "z")
	d := writeFile(t, root, "sub/d.txt", "q") // nested
	_ = writeFile(t, root, "sub/e.md", "w")

	got, err := Discover(root, Selector{Ext: "txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{a, b, d}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ext=txt: got\n%v\nwant\n%v", got, want)
	}
}

func TestDiscover_ByGlob(t *testing.T) {
	root := t.TempDir()
	_ = writeFile(t, root, "a.txt", "x")
	_ = writeFile(t, root, "c.md", "z")
	e := writeFile(t, root, "sub/e.md", "w")

	// Non-recursive glob from root: only *.md at sub/ level when pattern includes sub/
	got, err := Discover(root, Selector{Glob: filepath.Join("sub", "*.md")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{e}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("glob: got %v want %v", got, want)
	}
}

func TestDiscover_FilesAndDedupAndExclude(t *testing.T) {
	root := t.TempDir()
	a := writeFile(t, root, "a.txt", "x")
	// b := writeFile(t, root, "b.txt", "y") // removed unused variable
	c := writeFile(t, root, "c.txt", "z")
	_ = writeFile(t, root, "sub/keep.md", "w")

	// Mix explicit files + ext. Exclude b.txt and everything under sub/
	sel := Selector{
		Ext:     "txt",
		Files:   []string{"a.txt", "sub/keep.md"}, // a.txt will be deduped; keep.md will be excluded
		Exclude: []string{"b.*", "sub/*"},
	}
	got, err := Discover(root, sel)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect a.txt and c.txt only (b.* excluded; sub/* excluded)
	want := []string{a, c}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dedup/exclude: got\n%v\nwant\n%v", got, want)
	}
}

func TestDiscover_IgnoresNonRegular(t *testing.T) {
	root := t.TempDir()
	_ = writeFile(t, root, "a.txt", "x")
	// Create a directory that matches ext walk boundary conditions
	if err := os.MkdirAll(filepath.Join(root, "d.txt"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	got, err := Discover(root, Selector{Ext: "txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Ensure directory "d.txt" is not included
	for _, p := range got {
		if filepath.Base(p) == "d.txt" {
			t.Fatalf("non-regular directory was included: %s", p)
		}
	}

	// Symlink test (best-effort, may be skipped on Windows without privileges)
	link := filepath.Join(root, "link.txt")
	target := filepath.Join(root, "a.txt")
	if err := os.Symlink(target, link); err == nil {
		got2, err2 := Discover(root, Selector{Files: []string{"link.txt"}})
		if err2 != nil {
			t.Fatalf("unexpected error: %v", err2)
		}
		if len(got2) != 0 {
			t.Fatalf("symlink should be ignored as non-regular, got: %v", got2)
		}
	} else if runtime.GOOS != "windows" {
		// On non-Windows, symlink creation should usually succeed
		t.Logf("symlink creation failed (non-fatal): %v", err)
	}
}

func TestDiscover_NoMatches_ReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	_ = writeFile(t, root, "a.txt", "x")
	got, err := Discover(root, Selector{Ext: "md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no matches, got %v", got)
	}
}

func TestDiscover_ExtWithDot_Normalizes(t *testing.T) {
	root := t.TempDir()
	a := writeFile(t, root, "a.txt", "x")
	got, err := Discover(root, Selector{Ext: ".txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != a {
		t.Fatalf("ext with dot: got %v want [%s]", got, a)
	}
}

func TestDiscover_InvalidGlob_joinsErrorButKeepsOtherResults(t *testing.T) {
	root := t.TempDir()
	a := writeFile(t, root, "a.txt", "x")
	// Invalid pattern (per filepath.Glob rules) to force an error
	sel := Selector{Glob: "[", Files: []string{"a.txt"}}
	got, err := Discover(root, sel)
	if err == nil {
		t.Fatalf("expected joined error from invalid glob, got nil")
	}
	if len(got) != 1 || got[0] != a {
		t.Fatalf("results should include valid files despite glob error: got %v want [%s]", got, a)
	}
}

func TestDiscover_Files_NonexistentIgnored(t *testing.T) {
	root := t.TempDir()
	a := writeFile(t, root, "a.txt", "x")
	got, err := Discover(root, Selector{Files: []string{"a.txt", "missing.txt"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != a {
		t.Fatalf("nonexistent should be ignored: got %v want [%s]", got, a)
	}
}

func TestDiscover_AbsoluteGlob(t *testing.T) {
	root := t.TempDir()
	f := writeFile(t, root, "a.txt", "x")
	absGlob := f // exact absolute path acts like a glob match
	got, err := Discover(root, Selector{Glob: absGlob})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != f {
		t.Fatalf("abs glob: got %v want [%s]", got, f)
	}
}

func TestDiscover_AbsoluteExclude(t *testing.T) {
	root := t.TempDir()
	a := writeFile(t, root, "a.txt", "x")
	b := writeFile(t, root, "b.txt", "y")
	got, err := Discover(root, Selector{Ext: "txt", Exclude: []string{a}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != b {
		t.Fatalf("abs exclude: got %v want [%s]", got, b)
	}
}

func TestDiscover_RootEmpty_NormalizesToDot(t *testing.T) {
	cwd, _ := os.Getwd()
	root := t.TempDir()
	aRel := "a.txt"
	_ = writeFile(t, root, aRel, "x")
	// Run in root so relative selectors resolve there
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	got, err := Discover("", Selector{Files: []string{aRel}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || filepath.Base(got[0]) != aRel {
		t.Fatalf("root empty: got %v", got)
	}
}

func TestDiscover_DedupAcrossSelectors(t *testing.T) {
	root := t.TempDir()
	a := writeFile(t, root, "a.txt", "x")
	got, err := Discover(root, Selector{Ext: "txt", Files: []string{"a.txt"}, Glob: "*.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != a {
		t.Fatalf("dedup across selectors: got %v want [%s]", got, a)
	}
}

func TestDiscover_WalkDirPermissionError_Joined(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission semantics differ on Windows")
	}
	root := t.TempDir()
	_ = writeFile(t, root, "a.txt", "x")
	denied := filepath.Join(root, "denied")
	if err := os.MkdirAll(denied, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Remove all permissions to trigger WalkDir error when entering
	if err := os.Chmod(denied, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(denied, 0o700) })
	_, err := Discover(root, Selector{Ext: "txt"})
	if err == nil {
		t.Fatalf("expected joined error from WalkDir, got nil")
	}
}

func TestDiscover_HiddenFilesByExt(t *testing.T) {
	root := t.TempDir()
	a := writeFile(t, root, ".env", "x")
	got, err := Discover(root, Selector{Ext: "env"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != a {
		t.Fatalf("hidden ext match: got %v want [%s]", got, a)
	}
}

func TestDiscover_SymlinkIgnored_InGlobAndExt(t *testing.T) {
	root := t.TempDir()
	real := writeFile(t, root, "real.txt", "x")
	sy := filepath.Join(root, "link.txt")
	if err := os.Symlink(real, sy); err != nil {
		if runtime.GOOS != "windows" {
			t.Skipf("symlink unsupported here: %v", err)
		}
	}
	// Ext should include only real, not symlink
	gotExt, err := Discover(root, Selector{Ext: "txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotExt) != 1 || gotExt[0] != real {
		t.Fatalf("ext with symlink: got %v want [%s]", gotExt, real)
	}
	// Glob matching both names should still filter out symlink
	gotGlob, err := Discover(root, Selector{Glob: filepath.Join(root, "*.txt")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotGlob) != 1 || gotGlob[0] != real {
		t.Fatalf("glob with symlink: got %v want [%s]", gotGlob, real)
	}
}
