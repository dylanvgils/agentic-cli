// Package marketplace syncs git-based plugin marketplace repos onto the host.
package marketplace

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/git"
)

// MarketplacesDirName is the subdirectory under $TOOL_HOME where synced marketplace clones live.
const MarketplacesDirName = "marketplaces"

// safeSlugPattern matches characters unsafe for a filesystem path segment.
var safeSlugPattern = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// gitClone and gitFetchReset are test-stubbable indirections over the git package.
var (
	gitClone      = git.Clone
	gitFetchReset = git.FetchReset
)

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

// CloneDirName hashes url into a filesystem-safe dir name shared by any project referencing that url.
func CloneDirName(url string) string {
	sum := sha256.Sum256([]byte(url))
	return urlSlug(url) + "-" + hex.EncodeToString(sum[:4])
}

// urlSlug extracts a filesystem-safe label from url's last path segment.
func urlSlug(url string) string {
	trimmed := strings.TrimSuffix(strings.TrimRight(url, "/"), ".git")
	if i := strings.LastIndexAny(trimmed, "/:"); i != -1 {
		trimmed = trimmed[i+1:]
	}
	if trimmed == "" {
		return "marketplace"
	}
	return safeSlugPattern.ReplaceAllString(trimmed, "-")
}

// Sync clones each entry (if missing) or fetches+resets it (if present), in order.
func Sync(entries []Entry, dirFor func(e Entry) string) ([]Result, error) {
	if err := checkDuplicateNames(entries); err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(entries))
	for _, e := range entries {
		result, err := syncEntry(e, dirFor(e))
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	return results, nil
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
