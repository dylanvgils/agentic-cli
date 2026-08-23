// Package fswatch watches host directory trees for filesystem activity, to
// audit what a tool container does to bind-mounted host paths from the host
// side, without needing any privilege inside the container.
//
// The Watcher/New/Options/Logger/Entry API is identical across platforms;
// each host OS gets its own backend file: watcher_linux.go (inotify),
// watcher_darwin.go (kqueue), watcher_other.go (unsupported - Start errors).
package fswatch
