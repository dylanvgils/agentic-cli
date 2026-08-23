// Package fswatch's macOS backend, built on kqueue via golang.org/x/sys/unix.
//
// kqueue's EVFILT_VNODE only reports that a watched file or directory
// changed, not which entry - so this backend watches every file and
// directory individually and diffs directory listings on NOTE_WRITE to
// recover adds/removes. Consequence: OpOpen never fires on macOS, and
// OpWrite coalesces more coarsely than inotify's IN_CLOSE_WRITE.
//go:build darwin

package fswatch

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// wakeupIdent is the EVFILT_USER identifier Stop triggers to interrupt a blocked Kevent call.
const wakeupIdent = 1

// vnodeFflags is the kqueue EVFILT_VNODE mask used for every watch.
const vnodeFflags = unix.NOTE_WRITE | unix.NOTE_EXTEND | unix.NOTE_DELETE | unix.NOTE_RENAME

// Watcher watches a set of host directory trees for filesystem activity via
// kqueue, logging what it observes through a Logger.
type Watcher struct {
	kq      int
	logger  *Logger
	exclude []string
	roots   []string

	mu      sync.Mutex
	fdPath  map[uintptr]string         // watched fd -> path
	pathFd  map[string]*os.File        // path -> the open file backing its kevent registration
	dirList map[string]map[string]bool // path -> cached child-name set, directories only

	done     chan struct{}
	stopOnce sync.Once
}

// New creates a Watcher over roots, deduplicating and collapsing nested roots.
// It does not touch kqueue or the filesystem until Start is called.
func New(roots []string, logger *Logger, opts Options) *Watcher {
	return &Watcher{
		logger:  logger,
		exclude: opts.Exclude,
		roots:   collapseRoots(roots),
		fdPath:  make(map[uintptr]string),
		pathFd:  make(map[string]*os.File),
		dirList: make(map[string]map[string]bool),
		done:    make(chan struct{}),
	}
}

// Start opens the kqueue instance and begins watching every root. A root
// that fails to watch is logged as a Detail entry and skipped; only failure
// to initialize kqueue itself is fatal.
func (w *Watcher) Start() error {
	kq, err := unix.Kqueue()
	if err != nil {
		return fmt.Errorf("init kqueue: %w", err)
	}
	w.kq = kq

	wake := unix.Kevent_t{Ident: wakeupIdent, Filter: unix.EVFILT_USER, Flags: unix.EV_ADD | unix.EV_CLEAR}
	if _, err := unix.Kevent(w.kq, []unix.Kevent_t{wake}, nil, nil); err != nil {
		_ = unix.Close(kq)
		return fmt.Errorf("init kqueue wakeup: %w", err)
	}

	for _, root := range w.roots {
		w.addTree(root)
	}

	go w.loop()
	return nil
}

// Stop signals the event loop to exit and waits for it, up to a bound. Idempotent.
func (w *Watcher) Stop() {
	w.stopOnce.Do(func() {
		wake := unix.Kevent_t{Ident: wakeupIdent, Filter: unix.EVFILT_USER, Fflags: unix.NOTE_TRIGGER}
		_, _ = unix.Kevent(w.kq, []unix.Kevent_t{wake}, nil, nil)
		select {
		case <-w.done:
		case <-time.After(2 * time.Second):
		}

		w.mu.Lock()
		for _, f := range w.pathFd {
			_ = f.Close()
		}
		w.mu.Unlock()
		_ = unix.Close(w.kq)
	})
}

// addTree adds a watch for root and, if it is a directory, every
// non-excluded file and subdirectory beneath it.
func (w *Watcher) addTree(root string) {
	info, err := os.Lstat(root)
	if err != nil {
		w.logger.LogDetail(fmt.Sprintf("skip %s: %v", root, err))
		return
	}

	if !info.IsDir() {
		w.addWatch(root, false)
		return
	}

	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			w.logger.LogDetail(fmt.Sprintf("skip %s: %v", path, err))
			return nil
		}
		if d.IsDir() {
			if path != root && isExcludedDir(d.Name(), w.exclude) {
				return filepath.SkipDir
			}
			w.addWatch(path, true)
			return nil
		}
		w.addWatch(path, false)
		return nil
	})
}

func (w *Watcher) addWatch(path string, isDir bool) {
	f, err := os.Open(path)
	if err != nil {
		w.logger.LogDetail(fmt.Sprintf("watch %s: %v", path, err))
		return
	}

	ev := unix.Kevent_t{
		Ident:  uint64(f.Fd()),
		Filter: unix.EVFILT_VNODE,
		Flags:  unix.EV_ADD | unix.EV_CLEAR,
		Fflags: vnodeFflags,
	}
	if _, err := unix.Kevent(w.kq, []unix.Kevent_t{ev}, nil, nil); err != nil {
		w.logger.LogDetail(fmt.Sprintf("watch %s: %v", path, err))
		_ = f.Close()
		return
	}

	w.mu.Lock()
	w.fdPath[f.Fd()] = path
	w.pathFd[path] = f
	if isDir {
		w.dirList[path] = listNames(path)
	}
	w.mu.Unlock()
}

// forget drops path's watch (if any) and closes its fd, which removes the
// kqueue registration.
func (w *Watcher) forget(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	f, ok := w.pathFd[path]
	if !ok {
		return
	}
	delete(w.fdPath, f.Fd())
	delete(w.pathFd, path)
	delete(w.dirList, path)
	_ = f.Close()
}

func (w *Watcher) loop() {
	defer close(w.done)
	events := make([]unix.Kevent_t, 64)

	for {
		n, err := unix.Kevent(w.kq, nil, events, nil)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return
		}

		for _, ev := range events[:n] {
			if ev.Filter == unix.EVFILT_USER && ev.Ident == wakeupIdent {
				return // Stop was called
			}
			w.handle(ev)
		}
	}
}

func (w *Watcher) handle(ev unix.Kevent_t) {
	w.mu.Lock()
	path, ok := w.fdPath[uintptr(ev.Ident)]
	_, isWatchedDir := w.dirList[path]
	w.mu.Unlock()
	if !ok {
		return
	}

	switch {
	case ev.Fflags&(unix.NOTE_DELETE|unix.NOTE_RENAME) != 0:
		op := OpDelete
		if ev.Fflags&unix.NOTE_RENAME != 0 {
			op = OpRename
		}
		w.logger.Log(op, path, isWatchedDir)
		w.forget(path)
		w.rearmIfReplaced(path)
	case isWatchedDir && ev.Fflags&unix.NOTE_WRITE != 0:
		w.diffDir(path)
	case ev.Fflags&(unix.NOTE_WRITE|unix.NOTE_EXTEND) != 0:
		w.logger.Log(OpWrite, path, false)
	}
}

// rearmIfReplaced re-watches path if something now exists there, covering the
// atomic-save case where a rename replaces the inode but not the name.
func (w *Watcher) rearmIfReplaced(path string) {
	info, err := os.Lstat(path)
	if err != nil {
		return
	}

	isDir := info.IsDir()
	w.logger.Log(OpCreate, path, isDir)
	if isDir {
		w.addTree(path)
	} else {
		w.addWatch(path, false)
	}
}

// diffDir re-lists a watched directory whose NOTE_WRITE fired, comparing
// against the cached listing to recover which entries were added or removed.
func (w *Watcher) diffDir(path string) {
	newNames := listNames(path)

	w.mu.Lock()
	oldNames := w.dirList[path]
	w.mu.Unlock()

	for name := range newNames {
		if oldNames[name] {
			continue
		}
		child := filepath.Join(path, name)
		info, err := os.Lstat(child)
		isDir := err == nil && info.IsDir()
		if isDir && isExcludedDir(name, w.exclude) {
			continue
		}

		w.logger.Log(OpCreate, child, isDir)
		if isDir {
			w.addTree(child)
		} else {
			w.addWatch(child, false)
		}
	}

	for name := range oldNames {
		if newNames[name] {
			continue
		}
		child := filepath.Join(path, name)
		w.mu.Lock()
		_, wasDir := w.dirList[child]
		w.mu.Unlock()
		w.logger.Log(OpDelete, child, wasDir)
		w.forget(child)
	}

	w.mu.Lock()
	w.dirList[path] = newNames
	w.mu.Unlock()
}

// listNames returns the base names of dir's entries, or an empty set if it
// can't be read (best-effort, mirrors addWatch's error handling).
func listNames(dir string) map[string]bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return map[string]bool{}
	}
	names := make(map[string]bool, len(entries))
	for _, e := range entries {
		names[e.Name()] = true
	}
	return names
}
