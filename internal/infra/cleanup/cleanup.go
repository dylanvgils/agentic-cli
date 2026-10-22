// Package cleanup provides a helper for propagating deferred cleanup errors (Close, RemoveAll, etc.) without masking an earlier error.
package cleanup

// Capture runs fn in a defer and, if it errors and *err is not already set, assigns it to *err.
func Capture(err *error, fn func() error) {
	if ferr := fn(); ferr != nil && *err == nil {
		*err = ferr
	}
}
