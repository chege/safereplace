package apply

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAtomic_Basic(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := WriteAtomic(p, []byte("world"), Options{}); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "world" {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestWriteAtomic_Backup(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("orig"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := WriteAtomic(p, []byte("new"), Options{Backup: true}); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	// new content
	got, _ := os.ReadFile(p)
	if string(got) != "new" {
		t.Fatalf("unexpected content: %q", got)
	}
	// backup exists with old content
	bak := filepath.Join(dir, "a.txt.bak")
	data, err := os.ReadFile(bak)
	if err != nil {
		t.Fatalf("expected backup: %v", err)
	}
	if string(data) != "orig" {
		t.Fatalf("unexpected backup content: %q", data)
	}
}

func TestWriteAtomic_BackupUnique(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("first"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// create existing backups
	_ = os.WriteFile(filepath.Join(dir, "a.txt.bak"), []byte("x"), 0o644)

	if err := WriteAtomic(p, []byte("second"), Options{Backup: true}); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "second" {
		t.Fatalf("unexpected content: %q", got)
	}
	bak2 := filepath.Join(dir, "a.txt.bak.1")
	if _, err := os.Stat(bak2); err != nil {
		t.Fatalf("expected unique backup: %v", err)
	}
}

func TestWriteAtomic_NoSuchFile(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nofile.txt")
	err := WriteAtomic(missing, []byte("x"), Options{})
	if err == nil || !strings.Contains(err.Error(), "stat") {
		t.Fatalf("expected stat error, got %v", err)
	}
}
