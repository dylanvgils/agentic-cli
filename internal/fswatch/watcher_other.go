// Package fswatch's fallback for any host OS without a native backend wired
// up yet (currently: everything except Linux and macOS - notably Windows,
// which would need ReadDirectoryChangesW, an entirely different API from
// inotify/kqueue).
//go:build !linux && !darwin

package fswatch

import "fmt"

// Watcher is a no-op stand-in. Start always fails, so a caller that
// explicitly enabled filesystem auditing sees a clear error rather than
// silent inactivity.
type Watcher struct{}

// New returns a Watcher whose Start always fails - roots, logger, and opts
// are accepted (and ignored) only to keep the constructor's signature
// identical across every platform backend.
func New(_ []string, _ *Logger, _ Options) *Watcher {
	return &Watcher{}
}

// Start always returns an error: this host OS has no filesystem-watch
// backend implemented yet.
func (w *Watcher) Start() error {
	return fmt.Errorf("filesystem audit logging is not supported on this host OS yet")
}

// Stop is a no-op - Start never succeeded, so there is nothing to tear down.
func (w *Watcher) Stop() {}
