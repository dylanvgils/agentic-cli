//go:build windows

package platform

import "os"

// OpenNoFollow opens path read-only; Windows has no O_NOFOLLOW, so callers compare the opened file with what they checked.
func OpenNoFollow(path string) (*os.File, error) {
	return os.Open(path)
}
