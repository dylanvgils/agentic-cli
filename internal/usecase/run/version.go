package run

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	// minDockerEngine is the oldest Docker Engine a Docker run accepts; older versions are refused.
	minDockerEngine = "26.0"

	// minMicrosandbox is the oldest msb a microsandbox run accepts; older versions are refused.
	minMicrosandbox = "0.7.12"
)

// requireVersion refuses version of what when it is older than minimum or unreadable.
func requireVersion(what, version, minimum string) error {
	ok, err := versionAtLeast(version, minimum)
	if err != nil {
		return fmt.Errorf("read the %s version: %w", what, err)
	}
	if !ok {
		return fmt.Errorf("%s %s is older than the minimum %s; update it and run again", what, version, minimum)
	}
	return nil
}

// versionAtLeast reports whether dotted version have is at least minimum, ignoring a leading "v" and any suffix after a
// part's digits, so "26.1.3-ce" counts as 26.1.3.
func versionAtLeast(have, minimum string) (bool, error) {
	h, err := versionParts(have)
	if err != nil {
		return false, err
	}
	m, err := versionParts(minimum)
	if err != nil {
		return false, err
	}

	for i := range max(len(h), len(m)) {
		hv, mv := partAt(h, i), partAt(m, i)
		if hv != mv {
			return hv > mv, nil
		}
	}
	return true, nil
}

// versionParts parses the numeric parts of version.
func versionParts(version string) ([]int, error) {
	var parts []int
	for _, p := range strings.Split(strings.TrimPrefix(version, "v"), ".") {
		digits := p[:len(p)-len(strings.TrimLeft(p, "0123456789"))]
		n, err := strconv.Atoi(digits)
		if err != nil {
			return nil, fmt.Errorf("not a version: %q", version)
		}
		parts = append(parts, n)

		// A suffix ends the version, as in "1.2-rc.3"
		if digits != p {
			break
		}
	}
	return parts, nil
}

// partAt returns parts[i], or 0 past its end.
func partAt(parts []int, i int) int {
	if i < len(parts) {
		return parts[i]
	}
	return 0
}
