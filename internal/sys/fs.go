// The two filesystem operations that need more care than os.WriteFile.

package sys

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteAtomic writes a file through a temp file and a rename, so a crash
// mid-write cannot leave a half-written config behind.
func WriteAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".ghu-*")
	if err != nil {
		return fmt.Errorf("creating temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("writing %s: %w", tmpName, err)
	}
	if err := tmp.Chmod(mode); err != nil {
		return fmt.Errorf("setting mode on %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("renaming %s to %s: %w", tmpName, path, err)
	}
	return nil
}

// CurrentDir is the working directory with symlinks resolved: git resolves
// symlinks before matching gitdir patterns, so ghu must match on the real path
// too or it would disagree with git about which profile governs here.
func CurrentDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(wd); err == nil {
		return resolved, nil
	}
	return wd, nil
}
