// Package fswatch watches host directory trees for filesystem activity. It
// exists to support agentic's audit-logging feature: a Docker bind mount
// shares the host's underlying file, so a host-side watch on a mounted path
// observes all activity on it regardless of which mount namespace - host or
// container - touched it, with no privilege or capability required. This
// lets agentic audit what a hardened, capability-dropped tool container does
// to bind-mounted host paths without relaxing any of that container's own
// restrictions.
//
// The exported Watcher/New/Options/Logger/Entry API is identical across
// platforms; the mechanism backing it is not, and each host OS gets its own
// file: watcher_linux.go (inotify, full fidelity - open/write/create/delete/
// rename with the changed path given directly by the kernel), watcher_darwin.go
// (kqueue - lower fidelity: no open events, and a changed directory must be
// re-listed and diffed to recover which entry changed, since kqueue doesn't
// report that directly), and watcher_other.go (every other OS, notably
// Windows - not implemented yet; Start returns a clear error instead of
// silently doing nothing).
package fswatch
