// Package version compares dotted version strings.
package version

import (
	"fmt"
	"strconv"
	"strings"
)

// AtLeast reports whether dotted version have is at least minimum, ignoring a leading "v" and any suffix after a
// part's digits, so "26.1.3-ce" counts as 26.1.3.
func AtLeast(have, minimum string) (bool, error) {
	h, err := parts(have)
	if err != nil {
		return false, err
	}
	m, err := parts(minimum)
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

// parts parses the numeric parts of version.
func parts(version string) ([]int, error) {
	var nums []int
	for _, p := range strings.Split(strings.TrimPrefix(version, "v"), ".") {
		digits := p[:len(p)-len(strings.TrimLeft(p, "0123456789"))]
		n, err := strconv.Atoi(digits)
		if err != nil {
			return nil, fmt.Errorf("not a version: %q", version)
		}
		nums = append(nums, n)

		// A suffix ends the version, as in "1.2-rc.3"
		if digits != p {
			break
		}
	}
	return nums, nil
}

// partAt returns parts[i], or 0 past its end.
func partAt(parts []int, i int) int {
	if i < len(parts) {
		return parts[i]
	}
	return 0
}
