// Package marketplace syncs git-based plugin marketplace repos onto the host,
// so tool containers never run git or reach a git host themselves.
package marketplace

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// gitTimeout bounds each git invocation so a hanging remote can't block a run
// forever. A var, not a const, so tests can shrink it.
var gitTimeout = 45 * time.Second

// runGit is a test-stubbable indirection over `git <args...>`, inheriting the
// parent's environment (SSH_AUTH_SOCK, credential helpers, netrc) unmodified.
var runGit = func(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "git", args...).CombinedOutput()
}

// Entry is one configured marketplace.
type Entry struct {
	Name string
	URL  string
}

// Result is the outcome of syncing one Entry.
type Result struct {
	Entry   Entry
	Dir     string // host directory now containing the clone
	Stale   bool   // true if this run's fetch failed and Dir holds a previous clone
	Warning error  // non-nil iff Stale
}

// CheckGitAvailable returns an error if git is not on the host PATH.
func CheckGitAvailable() error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found on PATH: %w", err)
	}
	return nil
}

// Sync clones each entry (if missing) or fetches+resets it (if present), in
// order. A fetch/reset failure is tolerated (Stale=true, Warning set); a
// clone failure aborts Sync immediately.
func Sync(entries []Entry, dirFor func(name string) string) ([]Result, error) {
	if err := checkDuplicateNames(entries); err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(entries))
	for _, e := range entries {
		result, err := syncEntry(e, dirFor(e.Name))
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	return results, nil
}

// Prune removes every immediate subdirectory of baseDir whose name is not in
// keep, returning the names removed. A missing baseDir is not an error.
func Prune(baseDir string, keep []string) ([]string, error) {
	entries, err := os.ReadDir(baseDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	keepSet := make(map[string]bool, len(keep))
	for _, k := range keep {
		keepSet[k] = true
	}

	var removed []string
	for _, entry := range entries {
		if keepSet[entry.Name()] {
			continue
		}
		if err := os.RemoveAll(filepath.Join(baseDir, entry.Name())); err != nil {
			return removed, err
		}
		removed = append(removed, entry.Name())
	}

	return removed, nil
}

func syncEntry(e Entry, dir string) (Result, error) {
	exists, err := dirExists(dir)
	if err != nil {
		return Result{}, fmt.Errorf("marketplace %q: %w", e.Name, err)
	}

	if !exists {
		if err := gitClone(e.URL, dir); err != nil {
			_ = os.RemoveAll(dir)
			return Result{}, fmt.Errorf("marketplace %q: %w", e.Name, err)
		}
		return Result{Entry: e, Dir: dir}, nil
	}

	if err := gitFetchReset(dir); err != nil {
		return Result{Entry: e, Dir: dir, Stale: true, Warning: err}, nil
	}

	return Result{Entry: e, Dir: dir}, nil
}

func checkDuplicateNames(entries []Entry) error {
	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		if seen[e.Name] {
			return fmt.Errorf("duplicate marketplace name %q", e.Name)
		}
		seen[e.Name] = true
	}
	return nil
}

func dirExists(dir string) (bool, error) {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("%s exists and is not a directory", dir)
	}
	return true, nil
}

func gitClone(url, dir string) error {
	out, err := gitRun("clone", "--", url, dir)
	if err != nil {
		return fmt.Errorf("git clone: %w\n%s", err, out)
	}
	return nil
}

// gitFetchReset resets dir to match its upstream. `reset --hard` rather than
// `pull` avoids merge conflicts on a force-pushed repo - dir is a pure mirror
// with nothing to preserve.
func gitFetchReset(dir string) error {
	if out, err := gitRunIn(dir, "fetch"); err != nil {
		return fmt.Errorf("git fetch: %w\n%s", err, out)
	}
	if out, err := gitRunIn(dir, "reset", "--hard", "@{upstream}"); err != nil {
		return fmt.Errorf("git reset: %w\n%s", err, out)
	}
	return nil
}

func gitRunIn(dir string, args ...string) ([]byte, error) {
	return gitRun(append([]string{"-C", dir}, args...)...)
}

func gitRun(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	return runGit(ctx, args...)
}
