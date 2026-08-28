// Package fswatch's macOS backend, built on kqueue via golang.org/x/sys/unix; diffs directory listings on NOTE_WRITE to recover adds/removes, so OpOpen never fires on macOS.
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

// Watcher watches host directory trees for filesystem activity via kqueue.
type Watcher struct {
	kq      int
	logger  *Logger
	exclude []string
	roots   []string
	watches *watchSet

	done     chan struct{}
	stopOnce sync.Once
}

// watchSet is a mutex-protected table of active watches, keyed both by path and by fd.
type watchSet struct {
	mu      sync.Mutex
	fdPath  map[uintptr]string
	pathFd  map[string]*os.File
	dirList map[string]map[string]bool
}

// New creates a Watcher over roots; it touches nothing until Start is called.
func New(roots []string, logger *Logger, opts Options) *Watcher {
	return &Watcher{
		logger:  logger,
		exclude: opts.Exclude,
		roots:   collapseRoots(roots),
		watches: newWatchSet(),
		done:    make(chan struct{}),
	}
}

// Start opens kqueue and watches every root; a root that fails is logged and skipped.
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

		w.watches.closeAll()
		_ = unix.Close(w.kq)
	})
}

// addTree adds a watch for root and every non-excluded entry beneath it.
func (w *Watcher) addTree(root string) {
	walkTree(root, w.exclude, w.logger, w.addWatch)
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

	w.watches.add(path, f, isDir)
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
	path, isWatchedDir, ok := w.watches.lookup(uintptr(ev.Ident))
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
		w.watches.remove(path)
		w.rearmIfReplaced(path)
	case isWatchedDir && ev.Fflags&unix.NOTE_WRITE != 0:
		w.diffDir(path)
	case ev.Fflags&(unix.NOTE_WRITE|unix.NOTE_EXTEND) != 0:
		w.logger.Log(OpWrite, path, false)
	}
}

// rearmIfReplaced re-watches path if something now exists there (the atomic-save case).
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

// diffDir re-lists a watched directory and diffs against the cached listing for adds/removes.
func (w *Watcher) diffDir(path string) {
	newNames := listNames(path)
	oldNames := w.watches.dirListing(path)

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
		wasDir := w.watches.isDir(child)
		w.logger.Log(OpDelete, child, wasDir)
		w.watches.remove(child)
	}

	w.watches.setDirListing(path, newNames)
}

// newWatchSet returns an empty watchSet.
func newWatchSet() *watchSet {
	return &watchSet{
		fdPath:  make(map[uintptr]string),
		pathFd:  make(map[string]*os.File),
		dirList: make(map[string]map[string]bool),
	}
}

func (s *watchSet) add(path string, f *os.File, isDir bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fdPath[f.Fd()] = path
	s.pathFd[path] = f
	if isDir {
		s.dirList[path] = listNames(path)
	}
}

// remove drops path's watch (if any) and closes its fd, removing the kqueue registration.
func (s *watchSet) remove(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, ok := s.pathFd[path]
	if !ok {
		return
	}
	delete(s.fdPath, f.Fd())
	delete(s.pathFd, path)
	delete(s.dirList, path)
	_ = f.Close()
}

// lookup returns the path watched by fd, whether it's a directory, and whether fd is known.
func (s *watchSet) lookup(fd uintptr) (path string, isDir bool, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, ok = s.fdPath[fd]
	_, isDir = s.dirList[path]
	return path, isDir, ok
}

// isDir reports whether path is a currently watched directory.
func (s *watchSet) isDir(path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.dirList[path]
	return ok
}

// dirListing returns the cached child-name set for a watched directory, or nil if path isn't one.
func (s *watchSet) dirListing(path string) map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dirList[path]
}

func (s *watchSet) setDirListing(path string, names map[string]bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dirList[path] = names
}

// closeAll closes every watched file, tearing down its kqueue registration.
func (s *watchSet) closeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.pathFd {
		_ = f.Close()
	}
}

// listNames returns the base names of dir's entries, or an empty set if it can't be read.
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
