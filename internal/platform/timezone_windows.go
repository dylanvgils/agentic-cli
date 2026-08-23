//go:build windows

package platform

import (
	"os/exec"
	"strings"
)

// timezone resolves the host's IANA timezone name via tzutil, translating its Windows-specific name to the IANA equivalent.
func timezone() string {
	out, err := exec.Command("tzutil", "/g").Output()
	if err != nil {
		return ""
	}
	return resolveWindowsTimezone(strings.TrimSpace(string(out)))
}

// resolveWindowsTimezone maps a Windows timezone name to its IANA equivalent, kept pure so it's testable without shelling out to tzutil.
func resolveWindowsTimezone(winName string) string {
	return windowsToIANA[winName]
}
