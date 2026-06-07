package store

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// writeAtomic writes data to a temp file in the same directory, fsyncs it, then
// renames over the target — so a reader never sees a half-written file.
func writeAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// mustBeUnderRoot is the guardrail that keeps every mutating op inside the ocp
// store, so a bug can never delete or overwrite the user's live opencode dirs.
func mustBeUnderRoot(root, path string) error {
	rp, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	pp, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rp, pp)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("refusing to operate outside ocp root: %s", path)
	}
	return nil
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// removeIfExists deletes path, treating a symlink as a link (never following it
// into the shared base). Used before re-materializing a domain.
func removeIfExists(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return os.Remove(path)
	}
	if fi.IsDir() {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}

// backupIfReal removes a symlink as-is, but moves real (owned) data aside to a
// timestamped .bak so switching a domain back to shared never destroys edits.
func backupIfReal(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return os.Remove(path)
	}
	return os.Rename(path, fmt.Sprintf("%s.bak-%d", path, time.Now().Unix()))
}

// replicateDir copies src→dst preserving symlinks as symlinks (same target).
// Used to seed shared/skills from the live skills dir, whose entries are
// themselves symlinks into a skill store — we keep them light rather than
// duplicating the real contents.
func replicateDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		sp := filepath.Join(src, e.Name())
		dp := filepath.Join(dst, e.Name())
		fi, err := os.Lstat(sp)
		if err != nil {
			return err
		}
		switch {
		case fi.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(sp)
			if err != nil {
				return err
			}
			_ = os.Remove(dp)
			if err := os.Symlink(target, dp); err != nil {
				return err
			}
		case fi.IsDir():
			if err := replicateDir(sp, dp); err != nil {
				return err
			}
		default:
			if err := copyFile(sp, dp, fi.Mode().Perm()); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyTree deep-copies src→dst, resolving symlinks into real copies so the
// result is fully self-contained. Used when a domain goes linked→owned.
func copyTree(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		sp := filepath.Join(src, e.Name())
		dp := filepath.Join(dst, e.Name())
		fi, err := os.Stat(sp) // follow symlinks → resolve to real targets
		if err != nil {
			continue // skip dangling links
		}
		if fi.IsDir() {
			if err := copyTree(sp, dp); err != nil {
				return err
			}
		} else {
			if err := copyFile(sp, dp, fi.Mode().Perm()); err != nil {
				return err
			}
		}
	}
	return nil
}
