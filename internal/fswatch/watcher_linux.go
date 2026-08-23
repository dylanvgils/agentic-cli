// Package fswatch's Linux backend, built on inotify via golang.org/x/sys/unix.
//go:build linux

package fswatch

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// watchMask is the inotify event mask used for every watch. IN_ACCESS/IN_MODIFY
// are excluded as too noisy (fire per syscall); IN_DONT_FOLLOW/IN_ONLYDIR guard
// against watching through a symlink or a path that changes type mid-discovery.
const watchMask = unix.IN_OPEN | unix.IN_CLOSE_WRITE | unix.IN_CREATE | unix.IN_DELETE |
	unix.IN_MOVED_FROM | unix.IN_MOVED_TO | unix.IN_DELETE_SELF | unix.IN_MOVE_SELF |
	unix.IN_ISDIR | unix.IN_DONT_FOLLOW | unix.IN_ONLYDIR

// inotifyEventHeaderSize is sizeof(struct inotify_event) minus the trailing
// variable-length name.
const inotifyEventHeaderSize = 16

// rawEvent is one decoded inotify_event, before being mapped back to a path.
type rawEvent struct {
	wd   int32
	mask uint32
	name string
}

// Watcher watches a set of host directory trees for filesystem activity via
// inotify. Watches are per directory - inotify has no native recursion, so
// Watcher walks each root at Start and adds a watch under any new directory.
type Watcher struct {
	fd           int
	stopR, stopW int
	logger       *Logger
	exclude      []string
	roots        []string
	watches      *watchSet

	done     chan struct{}
	stopOnce sync.Once
}

// watchSet is a mutex-protected bidirectional map between an inotify watch
// descriptor and the path it watches.
type watchSet struct {
	mu     sync.Mutex
	byWd   map[int32]string
	byPath map[string]int32
}

// New creates a Watcher over roots, deduplicating and collapsing nested roots.
// It does not touch inotify or the filesystem until Start is called.
func New(roots []string, logger *Logger, opts Options) *Watcher {
	return &Watcher{
		logger:  logger,
		exclude: opts.Exclude,
		roots:   collapseRoots(roots),
		watches: newWatchSet(),
		done:    make(chan struct{}),
	}
}

// Start opens the inotify instance and begins watching every root. A root
// that fails to watch is logged as a Detail entry and skipped; only failure
// to initialize inotify (or its stop-signaling pipe) is fatal.
func (w *Watcher) Start() error {
	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC)
	if err != nil {
		return fmt.Errorf("init inotify: %w", err)
	}
	w.fd = fd

	var pipeFDs [2]int
	if err := unix.Pipe2(pipeFDs[:], unix.O_CLOEXEC); err != nil {
		_ = unix.Close(fd)
		return fmt.Errorf("init stop pipe: %w", err)
	}
	w.stopR, w.stopW = pipeFDs[0], pipeFDs[1]

	for _, root := range w.roots {
		w.addTree(root)
	}

	go w.loop()
	return nil
}

// Stop signals the event loop to exit via the stop pipe (safer than a
// concurrent fd close, which is unreliable on Linux mid-read) and waits for
// it, up to a bound. Idempotent.
func (w *Watcher) Stop() {
	w.stopOnce.Do(func() {
		_, _ = unix.Write(w.stopW, []byte{0})
		select {
		case <-w.done:
		case <-time.After(2 * time.Second):
		}
		_ = unix.Close(w.stopW)
		_ = unix.Close(w.stopR)
		_ = unix.Close(w.fd)
	})
}

// addTree adds a watch for root and, if it is a directory, every
// non-excluded subdirectory beneath it.
func (w *Watcher) addTree(root string) {
	info, err := os.Lstat(root)
	if err != nil {
		w.logger.LogDetail(fmt.Sprintf("skip %s: %v", root, err))
		return
	}

	if !info.IsDir() {
		w.addWatch(root)
		return
	}

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			w.logger.LogDetail(fmt.Sprintf("skip %s: %v", path, err))
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && isExcludedDir(d.Name(), w.exclude) {
			return filepath.SkipDir
		}
		w.addWatch(path)
		return nil
	})
}

func (w *Watcher) addWatch(path string) {
	wd, err := unix.InotifyAddWatch(w.fd, path, watchMask)
	if err != nil {
		w.logger.LogDetail(fmt.Sprintf("watch %s: %v", path, err))
		return
	}
	w.watches.add(int32(wd), path)
}

func (w *Watcher) loop() {
	defer close(w.done)
	buf := make([]byte, 64*1024)
	pollFDs := []unix.PollFd{
		{Fd: int32(w.fd), Events: unix.POLLIN},
		{Fd: int32(w.stopR), Events: unix.POLLIN},
	}

	for {
		pollFDs[0].Revents, pollFDs[1].Revents = 0, 0
		n, err := unix.Poll(pollFDs, -1)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return
		}
		if n == 0 {
			continue
		}
		if pollFDs[1].Revents&unix.POLLIN != 0 {
			return // Stop was called
		}
		if pollFDs[0].Revents&unix.POLLIN == 0 {
			continue
		}

		nRead, err := unix.Read(w.fd, buf)
		if nRead <= 0 || err != nil {
			return
		}
		for _, ev := range decodeEvents(buf[:nRead]) {
			w.handle(ev)
		}
	}
}

func (w *Watcher) handle(ev rawEvent) {
	if ev.mask&unix.IN_Q_OVERFLOW != 0 {
		w.logger.LogDetail("inotify queue overflow: some events were dropped")
		return
	}

	base, ok := w.watches.path(ev.wd)
	if !ok {
		return
	}

	path := base
	if ev.name != "" {
		path = filepath.Join(base, ev.name)
	}
	isDir := ev.mask&unix.IN_ISDIR != 0

	switch {
	case ev.mask&(unix.IN_DELETE_SELF|unix.IN_MOVE_SELF) != 0:
		w.watches.remove(ev.wd)
	case ev.mask&unix.IN_CREATE != 0:
		// Watch before logging, so activity right after creation can't be missed.
		if isDir {
			w.addTree(path)
		}
		w.logger.Log(OpCreate, path, isDir)
	case ev.mask&unix.IN_DELETE != 0:
		w.logger.Log(OpDelete, path, isDir)
	case ev.mask&(unix.IN_MOVED_FROM|unix.IN_MOVED_TO) != 0:
		w.logger.Log(OpRename, path, isDir)
	case ev.mask&unix.IN_OPEN != 0:
		w.logger.Log(OpOpen, path, isDir)
	case ev.mask&unix.IN_CLOSE_WRITE != 0:
		w.logger.Log(OpWrite, path, isDir)
	}
}

// newWatchSet returns an empty watchSet.
func newWatchSet() *watchSet {
	return &watchSet{
		byWd:   make(map[int32]string),
		byPath: make(map[string]int32),
	}
}

func (s *watchSet) add(wd int32, path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byWd[wd] = path
	s.byPath[path] = wd
}

func (s *watchSet) remove(wd int32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if path, ok := s.byWd[wd]; ok {
		delete(s.byWd, wd)
		delete(s.byPath, path)
	}
}

// path returns the watched path for wd, if any.
func (s *watchSet) path(wd int32) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, ok := s.byWd[wd]
	return path, ok
}

// decodeEvents decodes zero or more packed inotify_event records from buf.
func decodeEvents(buf []byte) []rawEvent {
	var events []rawEvent
	off := 0

	for off+inotifyEventHeaderSize <= len(buf) {
		wd := int32(binary.LittleEndian.Uint32(buf[off : off+4]))
		mask := binary.LittleEndian.Uint32(buf[off+4 : off+8])
		nameLen := int(binary.LittleEndian.Uint32(buf[off+12 : off+16]))

		nameStart := off + inotifyEventHeaderSize
		nameEnd := nameStart + nameLen
		if nameEnd > len(buf) {
			break
		}

		name := string(bytes.TrimRight(buf[nameStart:nameEnd], "\x00"))
		events = append(events, rawEvent{wd: wd, mask: mask, name: name})
		off = nameEnd
	}

	return events
}
