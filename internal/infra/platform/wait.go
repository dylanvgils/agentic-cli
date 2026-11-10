package platform

import "errors"

// ErrProcessGone is returned by OpenProcess for a pid that no longer runs.
var ErrProcessGone = errors.New("process is gone")
