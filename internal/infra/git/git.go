package git

import (
	"fmt"
	"os/exec"
	"slices"
)

// remoteProtocols limits clone and fetch to https and ssh (incl. scp-style user@host:path), so a URL can't read host
// paths (file) or fetch unauthenticated (git://); git already refuses ext by default.
var remoteProtocols = []string{
	"-c", "protocol.allow=never",
	"-c", "protocol.https.allow=always",
	"-c", "protocol.ssh.allow=always",
}

// Clone runs `git clone -- url dir` over https or ssh only.
func (c *Client) Clone(url, dir string) error {
	out, err := c.runIn("", slices.Concat(remoteProtocols, []string{"clone", "--", url, dir})...)
	if err != nil {
		return fmt.Errorf("git clone: %w\n%s", err, out)
	}
	return nil
}

// FetchReset resets dir to match its upstream over https or ssh only, tolerating a force-pushed repo.
func (c *Client) FetchReset(dir string) error {
	if out, err := c.runIn(dir, append(slices.Clone(remoteProtocols), "fetch")...); err != nil {
		return fmt.Errorf("git fetch: %w\n%s", err, out)
	}
	if out, err := c.runIn(dir, "reset", "--hard", "@{upstream}"); err != nil {
		return fmt.Errorf("git reset: %w\n%s", err, out)
	}
	return nil
}

// CheckAvailable returns an error if git is not on the host PATH.
func CheckAvailable() error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found on PATH: %w", err)
	}
	return nil
}
