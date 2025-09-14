package processor

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeTemp(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func TestLiteral_NoMatches(t *testing.T) {
	dir := t.TempDir()
	p := writeTemp(t, dir, "a.txt", "hello world")
	res, err := SubstituteLiteralFile(p, "xxx", "yyy")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Changed || res.Matches != 0 || res.Replacements != 0 {
		t.Fatalf("unexpected: %+v", res)
	}
	if res.Before != res.After {
		t.Fatalf("before/after mismatch")
	}
}

func TestLiteral_MatchesAndReplace(t *testing.T) {
	dir := t.TempDir()
	p := writeTemp(t, dir, "a.txt", "foo\nbar foo\n")
	res, err := SubstituteLiteralFile(p, "foo", "baz")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !res.Changed {
		t.Fatalf("should be changed")
	}
	if res.Matches != 2 || res.Replacements != 2 {
		t.Fatalf("counts wrong: %+v", res)
	}
	if want := "baz\nbar baz\n"; res.After != want {
		t.Fatalf("after wrong: %q", res.After)
	}
}

func TestLiteral_PreservesCRLF(t *testing.T) {
	dir := t.TempDir()
	// Use explicit CRLF content
	content := "foo\r\nbar\r\n"
	p := writeTemp(t, dir, "a.txt", content)
	res, err := SubstituteLiteralFile(p, "foo", "bar")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got, want := res.After, "bar\r\nbar\r\n"; got != want {
		t.Fatalf("eol changed: got %q want %q", got, want)
	}
}

func TestLiteral_BinaryIsSkipped(t *testing.T) {
	dir := t.TempDir()
	p := writeTemp(t, dir, "bin.dat", string([]byte{0x00, 0x01, 0x02}))
	_, err := SubstituteLiteralFile(p, "a", "b")
	if err == nil {
		t.Fatalf("expected error for binary file")
	}
}

func TestLiteral_LargeFile_Simple(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	dir := t.TempDir()
	// ~1MB text
	chunk := make([]byte, 1024)
	for i := range chunk {
		chunk[i] = 'a'
	}
	body := string(chunk)
	for i := 0; i < 1024; i++ {
		body += "\n" + string(chunk)
	}
	p := writeTemp(t, dir, "big.txt", body)
	res, err := SubstituteLiteralFile(p, "aaa", "bbb")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !res.Changed {
		t.Fatalf("expected changes on large file")
	}
}

func TestLiteral_PathWithUnicode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("path unicode quirk on CI")
	}
	dir := t.TempDir()
	p := writeTemp(t, dir, "føø/å.txt", "foo")
	res, err := SubstituteLiteralFile(p, "foo", "bar")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.After != "bar" {
		t.Fatalf("wrong after: %q", res.After)
	}
}

func TestLiteral_EmptyPattern_NoOp(t *testing.T) {
	dir := t.TempDir()
	p := writeTemp(t, dir, "a.txt", "some content")
	res, err := SubstituteLiteralFile(p, "", "xxx")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Changed || res.Matches != 0 || res.Replacements != 0 {
		t.Fatalf("empty pattern should be no-op: %+v", res)
	}
	if res.Before != res.After {
		t.Fatalf("before/after mismatch on empty pattern")
	}
}

func TestLiteral_ReplacementSameAsPattern_NoChange(t *testing.T) {
	dir := t.TempDir()
	p := writeTemp(t, dir, "a.txt", "foo foo")
	res, err := SubstituteLiteralFile(p, "foo", "foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Changed {
		t.Fatalf("replacement same as pattern should not change content")
	}
	if res.Matches == 0 || res.Replacements == 0 {
		t.Fatalf("expected matches and replacements to count occurrences; got: %+v", res)
	}
	if res.Before != res.After {
		t.Fatalf("content should be identical when replacement equals pattern")
	}
}

func TestLiteral_MissingFile_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope.txt")
	_, err := SubstituteLiteralFile(missing, "a", "b")
	if err == nil {
		t.Fatalf("expected error for missing file")
	}
}

func TestLiteral_EmptyFile_NoChange(t *testing.T) {
	dir := t.TempDir()
	p := writeTemp(t, dir, "empty.txt", "")
	res, err := SubstituteLiteralFile(p, "a", "b")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Changed || res.Matches != 0 || res.Replacements != 0 {
		t.Fatalf("empty file should yield no changes: %+v", res)
	}
	if res.Before != "" || res.After != "" {
		t.Fatalf("expected empty before/after")
	}
}
