package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"safereplace/internal/testutil"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	binDir := t.TempDir()
	bin := filepath.Join(binDir, exeName("safereplace-test"))
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

func exeName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func TestCLI_DryRun_Ext(t *testing.T) {
	bin := buildBinary(t)
	work := t.TempDir()
	// Create files: one with matches, one without, and one different ext.
	a := testutil.WriteFile(t, work, "a.txt", "foo\nkeep\nfoo\n")
	_ = testutil.WriteFile(t, work, "c.txt", "nope\n")
	_ = testutil.WriteFile(t, work, "skip.md", "foo\n")

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(bin, "--pattern", "foo", "--replace", "bar", "--ext", "txt", "--dry-run", "--no-color")
	cmd.Dir = work
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	// Expect exit code 1 (changes detected). On non-zero, Run returns *ExitError.
	if err == nil {
		// ok, but then it must be exit 0; which would be wrong here
		if cmd.ProcessState.ExitCode() != 1 {
			t.Fatalf("expected exit code 1, got %d", cmd.ProcessState.ExitCode())
		}
	} else {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			if ee.ExitCode() != 1 {
				t.Fatalf("expected exit code 1, got %d; stderr=%s", ee.ExitCode(), stderr.String())
			}
		}
	}

	out := stdout.String()
	if !strings.Contains(out, "file: "+a) {
		t.Fatalf("missing per-file header for a.txt; out=\n%s", out)
	}
	if !strings.Contains(out, "--- before\n") || !strings.Contains(out, "+++ after\n") {
		t.Fatalf("missing diff headers; out=\n%s", out)
	}
	if !strings.Contains(out, "-foo") || !strings.Contains(out, "+bar") {
		t.Fatalf("missing diff lines; out=\n%s", out)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestCLI_NoChanges_Exit0(t *testing.T) {
	bin := buildBinary(t)
	work := t.TempDir()
	_ = testutil.WriteFile(t, work, "a.txt", "hello\nworld\n")

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(bin, "--pattern", "zzz", "--replace", "qqq", "--ext", "txt", "--dry-run", "--no-color")
	cmd.Dir = work
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			if ee.ExitCode() != 0 {
				t.Fatalf("expected exit 0, got %d; stderr=%s", ee.ExitCode(), stderr.String())
			}
		}
	} else if cmd.ProcessState.ExitCode() != 0 {
		t.Fatalf("expected exit 0, got %d", cmd.ProcessState.ExitCode())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got: %s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got: %s", stderr.String())
	}
}

func TestCLI_Apply_WritesAndBackup(t *testing.T) {
	bin := buildBinary(t)
	work := t.TempDir()
	p := testutil.WriteFile(t, work, "a.txt", "foo\n")

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(bin, "--pattern", "foo", "--replace", "bar", "--ext", "txt", "--no-color", "--dry-run=false", "--backup")
	cmd.Dir = work
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	// Expect exit code 1 (changes applied)
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			if ee.ExitCode() != 1 {
				t.Fatalf("expected exit code 1, got %d; stderr=%s", ee.ExitCode(), stderr.String())
			}
		}
	} else if cmd.ProcessState.ExitCode() != 1 {
		t.Fatalf("expected exit code 1, got %d", cmd.ProcessState.ExitCode())
	}

	// Verify content replaced
	data, rerr := os.ReadFile(p)
	if rerr != nil {
		t.Fatalf("read after apply: %v", rerr)
	}
	if string(data) != "bar\n" {
		t.Fatalf("content not applied: %q", data)
	}
	// Verify backup
	bak := filepath.Join(work, "a.txt.bak")
	bdata, berr := os.ReadFile(bak)
	if berr != nil {
		t.Fatalf("missing backup: %v", berr)
	}
	if string(bdata) != "foo\n" {
		t.Fatalf("backup content wrong: %q", bdata)
	}
}
