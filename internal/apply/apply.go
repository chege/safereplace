package apply

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Options controls how file application is performed.
// BackupSuffix is used only when Backup is true; if empty, ".bak" is used.
// Writes are done to a temp file in the same directory and then atomically renamed.
// File mode (permissions) of the original is preserved on the new file.
// The parent directory is fsynced on platforms that support it (best-effort on Windows).
type Options struct {
	Backup       bool
	BackupSuffix string
}

// WriteAtomic writes data to path safely:
//  1. stat the existing file (required)
//  2. optionally create a backup (unique name)
//  3. write to a temp file in the same dir, fsync, close
//  4. atomic rename over the original
//  5. fsync the parent directory (best-effort)
func WriteAtomic(path string, data []byte, opts Options) error {
	// 1) stat original (must exist; preserving mode)
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("apply: stat: %w", err)
	}
	mode := info.Mode().Perm()

	dir := filepath.Dir(path)
	base := filepath.Base(path)

	// 2) optional backup
	if opts.Backup {
		bak := opts.BackupSuffix
		if bak == "" {
			bak = ".bak"
		}
		backupPath, berr := uniqueBackupPath(dir, base, bak)
		if berr != nil {
			return berr
		}
		if err := copyFile(path, backupPath, mode); err != nil {
			return fmt.Errorf("apply: backup: %w", err)
		}
	}

	// 3) write temp in same dir
	tf, err := os.CreateTemp(dir, base+".tmp-*")
	if err != nil {
		return fmt.Errorf("apply: temp: %w", err)
	}
	renamed := false
	defer func(name string) {
		if !renamed {
			_ = os.Remove(name)
		}
	}(tf.Name())
	if _, err := tf.Write(data); err != nil {
		return errors.Join(fmt.Errorf("apply: write temp: %w", err), tf.Close())
	}
	if err := tf.Chmod(mode); err != nil {
		return errors.Join(fmt.Errorf("apply: chmod temp: %w", err), tf.Close())
	}
	if err := tf.Sync(); err != nil {
		return errors.Join(fmt.Errorf("apply: fsync temp: %w", err), tf.Close())
	}
	if err := tf.Close(); err != nil {
		return fmt.Errorf("apply: close temp: %w", err)
	}

	// 4) atomic replace
	if err := os.Rename(tf.Name(), path); err != nil {
		return fmt.Errorf("apply: rename: %w", err)
	}
	renamed = true

	// 5) fsync parent dir (best effort; may not work on Windows)
	_ = syncDir(dir) // best-effort; ignore error

	return nil
}

func uniqueBackupPath(dir, base, suffix string) (string, error) {
	cand := filepath.Join(dir, base+suffix)
	if _, err := os.Lstat(cand); os.IsNotExist(err) {
		return cand, nil
	}
	for i := 1; i < 1000; i++ {
		p := filepath.Join(dir, fmt.Sprintf("%s%s.%d", base, suffix, i))
		if _, err := os.Lstat(p); os.IsNotExist(err) {
			return p, nil
		}
	}
	return "", fmt.Errorf("apply: backup: too many existing backups for %s", base)
}

func copyFile(src, dst string, mode os.FileMode) error {
	r, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	w, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() { _ = w.Close() }()
	if _, err := io.Copy(w, r); err != nil {
		return err
	}
	return w.Sync()
}

func syncDir(dir string) error {
	df, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() { _ = df.Close() }()
	return df.Sync()
}
