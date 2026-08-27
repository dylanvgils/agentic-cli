package migrations

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// relocatedToolDirs is a historical snapshot, not tools.Names() - it must not track future tool changes.
var relocatedToolDirs = []string{"claude", "copilot", "opencode"}

// MoveToolAndProxyDirs relocates TOOL_HOME/<tool> to TOOL_HOME/tools/<tool> and TOOL_HOME/proxy/*.jsonl to TOOL_HOME/logs/proxy_*.jsonl. Safe to retry after a partial failure.
func MoveToolAndProxyDirs(toolHome string) error {
	if err := moveToolDirs(toolHome); err != nil {
		return err
	}
	return moveProxyLogs(toolHome)
}

func moveToolDirs(toolHome string) error {
	toolsDir := filepath.Join(toolHome, tools.ToolsDirName)
	if err := os.MkdirAll(toolsDir, 0o750); err != nil {
		return fmt.Errorf("create %s: %w", toolsDir, err)
	}

	for _, name := range relocatedToolDirs {
		if err := moveIfNeeded(filepath.Join(toolHome, name), filepath.Join(toolsDir, name)); err != nil {
			return fmt.Errorf("move %s: %w", name, err)
		}
	}
	return nil
}

func moveProxyLogs(toolHome string) error {
	logsDir := filepath.Join(toolHome, config.LogsDirName)
	if err := os.MkdirAll(logsDir, 0o750); err != nil {
		return fmt.Errorf("create %s: %w", logsDir, err)
	}

	proxyDir := filepath.Join(toolHome, "proxy")
	entries, err := os.ReadDir(proxyDir)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}

		src := filepath.Join(proxyDir, entry.Name())
		dst := filepath.Join(logsDir, proxy.LogFilePrefix+entry.Name())
		if err := moveIfNeeded(src, dst); err != nil {
			return fmt.Errorf("move proxy log %s: %w", entry.Name(), err)
		}
	}

	return removeIfEmpty(proxyDir)
}

// moveIfNeeded renames src to dst, a no-op if dst already exists or src is already gone.
func moveIfNeeded(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	return os.Rename(src, dst)
}

// removeIfEmpty removes dir only if it exists and is empty.
func removeIfEmpty(dir string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if len(entries) > 0 {
		return nil
	}
	return os.Remove(dir)
}
