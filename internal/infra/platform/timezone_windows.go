//go:build windows

package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// timezone resolves the host's IANA timezone name via tzutil, translating its Windows-specific name to the IANA equivalent.
func timezone() string {
	// The system copy, never one found on PATH or relative to the cwd
	root := os.Getenv("SystemRoot")
	if !filepath.IsAbs(root) {
		return ""
	}
	out, err := exec.Command(filepath.Join(root, "System32", "tzutil.exe"), "/g").Output()
	if err != nil {
		return ""
	}
	return resolveWindowsTimezone(strings.TrimSpace(string(out)))
}

// resolveWindowsTimezone maps a Windows timezone name to its IANA equivalent, kept pure so it's testable without shelling out to tzutil.
func resolveWindowsTimezone(winName string) string {
	return windowsToIANA[winName]
}
