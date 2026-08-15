package git

import (
	"fmt"
	"os/exec"
)

// CheckAvailable returns an error if git is not on the host PATH.
func CheckAvailable() error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found on PATH: %w", err)
	}
	return nil
}

// Clone runs `git clone -- url dir`.
func Clone(url, dir string) error {
	out, err := runWithTimeout("clone", "--", url, dir)
	if err != nil {
		return fmt.Errorf("git clone: %w\n%s", err, out)
	}
	return nil
}

// FetchReset resets dir to match its upstream, tolerating a force-pushed repo.
func FetchReset(dir string) error {
	if out, err := runIn(dir, "fetch"); err != nil {
		return fmt.Errorf("git fetch: %w\n%s", err, out)
	}
	if out, err := runIn(dir, "reset", "--hard", "@{upstream}"); err != nil {
		return fmt.Errorf("git reset: %w\n%s", err, out)
	}
	return nil
}
