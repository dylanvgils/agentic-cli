package sandbox

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dylanvgils/agentic-cli/internal/infra/hostfs"
)

// runTempDirPrefix starts every run temp dir's name.
const runTempDirPrefix = "agentic-run-"

// RunTempDir returns run id's private dir in the OS temp dir, holding the host files a run must not leave behind.
func RunTempDir(id RunID) string {
	return filepath.Join(os.TempDir(), runTempDirPrefix+id.String())
}

// CreateRunTempDir creates run id's temp dir, failing if it exists, so nobody else can have prepared it.
func CreateRunTempDir(id RunID) error {
	if _, err := ParseRunID(id.String()); err != nil {
		return err
	}
	if err := hostfs.Private.Mkdir(RunTempDir(id)); err != nil {
		return fmt.Errorf("create run temp dir: %w", err)
	}
	return nil
}

// CreateRunFile creates name in run id's temp dir, readable only by this user and failing if it exists.
func CreateRunFile(id RunID, name string) (*os.File, error) {
	return os.OpenFile(filepath.Join(RunTempDir(id), name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, hostfs.Private.File())
}

// RemoveRunTempDir removes run id's temp dir with its files; a missing one is fine.
func RemoveRunTempDir(id RunID) error {
	if _, err := ParseRunID(id.String()); err != nil {
		return err
	}
	if err := os.RemoveAll(RunTempDir(id)); err != nil {
		return fmt.Errorf("remove run temp dir: %w", err)
	}
	return nil
}
