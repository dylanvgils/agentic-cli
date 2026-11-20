package logging

import (
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// DirName is the subdirectory under $AGENTIC_HOME holding one dated log dir per run or command.
const DirName = "logs"

// logTimeLayout stamps a log dir with its local start time.
const logTimeLayout = "20060102-150405"

// logDirPattern matches Dir names, capturing the start time.
var logDirPattern = regexp.MustCompile(`^(\d{8}-\d{6})-.+$`)

// Dir returns the log dir of name (a run ID or command), started at start, under home.
func Dir(home, name string, start time.Time) string {
	return filepath.Join(home, DirName, start.Format(logTimeLayout)+"-"+name)
}

// Prune removes the log dirs and loose log files under home older than maxAge (<= 0 removes all), ignoring errors.
func Prune(home string, maxAge time.Duration) {
	dir := filepath.Join(home, DirName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		started, ok := logEntryTime(entry)
		if !ok || (maxAge > 0 && started.After(cutoff)) {
			continue
		}
		_ = os.RemoveAll(filepath.Join(dir, entry.Name()))
	}
}

// logEntryTime returns a log dir's start time from its name or a loose file's mtime; false for anything else.
func logEntryTime(entry os.DirEntry) (time.Time, bool) {
	if !entry.IsDir() {
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return time.Time{}, false
		}
		return info.ModTime(), true
	}

	m := logDirPattern.FindStringSubmatch(entry.Name())
	if m == nil {
		return time.Time{}, false
	}
	started, err := time.ParseInLocation(logTimeLayout, m[1], time.Local)
	return started, err == nil
}
