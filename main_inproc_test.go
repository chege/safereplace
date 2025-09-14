package main

import (
	"bytes"
	"os"
	"path/filepath"
	"safereplace/internal/cli"
	"safereplace/internal/testutil"
	"testing"
)

func TestRun_DryRun_ChangesExit1(t *testing.T) {
	work := t.TempDir()
	p := testutil.WriteFile(t, work, "a.txt", "foo\nkeep\nfoo\n")

	var out, err bytes.Buffer
	code := cli.Run([]string{"--pattern", "foo", "--replace", "bar", "--ext", "txt", "--no-color", "--files", p}, &out, &err)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d; stderr=%s", code, err.String())
	}
	got := out.String()
	if !bytes.Contains([]byte(got), []byte("file: "+p)) {
		t.Fatalf("missing file header; out=\n%s", got)
	}
	if !bytes.Contains([]byte(got), []byte("-foo")) || !bytes.Contains([]byte(got), []byte("+bar")) {
		t.Fatalf("expected diff lines; out=\n%s", got)
	}
	if err.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", err.String())
	}
}

func TestRun_NoChanges_Exit0(t *testing.T) {
	work := t.TempDir()
	_ = testutil.WriteFile(t, work, "a.txt", "hello\nworld\n")

	var out, err bytes.Buffer
	code := cli.Run([]string{"--pattern", "zzz", "--replace", "qqq", "--ext", "txt", "--no-color", "--files", filepath.Join(work, "a.txt")}, &out, &err)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr=%s", code, err.String())
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout, got: %s", out.String())
	}
	if err.Len() != 0 {
		t.Fatalf("expected no stderr, got: %s", err.String())
	}
}

func TestRun_Apply_WritesAndBackup(t *testing.T) {
	work := t.TempDir()
	p := testutil.WriteFile(t, work, "a.txt", "foo\n")

	var out, err bytes.Buffer
	code := cli.Run([]string{"--pattern", "foo", "--replace", "bar", "--ext", "txt", "--no-color", "--dry-run=false", "--backup", "--files", p}, &out, &err)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d; stderr=%s", code, err.String())
	}
	data, rerr := os.ReadFile(p)
	if rerr != nil {
		t.Fatalf("read after apply: %v", rerr)
	}
	if string(data) != "bar\n" {
		t.Fatalf("content not applied: %q", data)
	}
	bak := filepath.Join(work, "a.txt.bak")
	bdata, berr := os.ReadFile(bak)
	if berr != nil {
		t.Fatalf("missing backup: %v", berr)
	}
	if string(bdata) != "foo\n" {
		t.Fatalf("backup content wrong: %q", bdata)
	}
}
