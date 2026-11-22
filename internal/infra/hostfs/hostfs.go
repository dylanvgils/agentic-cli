// Package hostfs creates agentic's files and dirs on the host, with the mode of a permission class.
package hostfs

import (
	"fmt"
	"os"
	"path/filepath"
)

// Class is who may read the files and dirs agentic creates.
type Class int

const (
	// Data is agentic's own state, such as $AGENTIC_HOME: this user and its group.
	Data Class = iota
	// Private is per-run or secret content: this user only.
	Private
	// Shared is read by a container running as another user, such as the DinD sidecar; keep it under a Private dir.
	Shared
)

// Dir returns c's dir mode.
func (c Class) Dir() os.FileMode {
	switch c {
	case Data:
		return 0o750
	case Private:
		return 0o700
	case Shared:
		return 0o755
	default:
		panic(fmt.Sprintf("hostfs: unknown class %d", c))
	}
}

// File returns c's file mode.
func (c Class) File() os.FileMode {
	switch c {
	case Data:
		return 0o640
	case Private:
		return 0o600
	case Shared:
		return 0o644
	default:
		panic(fmt.Sprintf("hostfs: unknown class %d", c))
	}
}

// Mkdir creates dir with c's mode, failing if it exists.
func (c Class) Mkdir(dir string) error {
	return os.Mkdir(dir, c.Dir())
}

// MkdirAll creates dir and any missing parents with c's mode; existing dirs keep theirs.
func (c Class) MkdirAll(dir string) error {
	return os.MkdirAll(dir, c.Dir())
}

// WriteFile writes data to path, creating it with c's mode; an existing file keeps its mode.
func (c Class) WriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, c.File())
}

// ReplaceFile writes data to a temp file next to path and renames it over path, so a concurrent reader never sees a partial file; path gets c's mode.
func (c Class) ReplaceFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Chmod(c.File())
	}
	if err != nil {
		_ = tmp.Close()
		return err
	}

	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
