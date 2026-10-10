//go:build windows

package platform

import "os"

// OpenInRoot opens name in root read-only; root follows links that stay inside it, so callers compare the opened file with what they checked.
func OpenInRoot(root *os.Root, name string) (*os.File, error) {
	return root.Open(name)
}
