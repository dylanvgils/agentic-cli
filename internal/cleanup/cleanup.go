// Package cleanup provides a helper for propagating deferred cleanup errors
// (Close, RemoveAll, etc.) without masking an earlier error.
package cleanup

// Capture runs fn and, if it returns a non-nil error and *err is not already
// set, assigns it to *err. Use it in a defer to propagate cleanup failures
// without masking an earlier error.
func Capture(err *error, fn func() error) {
	if ferr := fn(); ferr != nil && *err == nil {
		*err = ferr
	}
}
