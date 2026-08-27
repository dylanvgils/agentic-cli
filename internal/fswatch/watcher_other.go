// Package fswatch's fallback for any host OS without a native backend wired up yet (e.g. Windows).
//go:build !linux && !darwin

package fswatch

import "fmt"

// Watcher is a no-op stand-in; Start always fails rather than silently doing nothing.
type Watcher struct{}

// New returns a Watcher whose Start always fails; args are ignored, kept only for signature parity.
func New(_ []string, _ *Logger, _ Options) *Watcher {
	return &Watcher{}
}

// Start always returns an error: this host OS has no filesystem-watch backend yet.
func (w *Watcher) Start() error {
	return fmt.Errorf("filesystem audit logging is not supported on this host OS yet")
}

// Stop is a no-op - Start never succeeded, so there is nothing to tear down.
func (w *Watcher) Stop() {}
