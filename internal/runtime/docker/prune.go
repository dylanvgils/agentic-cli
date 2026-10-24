package docker

import "strings"

// PruneImages removes dangling Docker images created by agentic builds and returns the space reclaimed.
func (c *Client) PruneImages() (string, error) {
	out, err := c.run("image", "prune",
		arg("force"),
		labelFilter(LabelProject, LabelProjectVal))
	if err != nil {
		return "", err
	}

	return reclaimedSpace(out), nil
}

// PruneBuildCache removes all unused BuildKit cache and returns the space reclaimed; cache records carry no labels,
// so it can't be limited to agentic's builds.
func (c *Client) PruneBuildCache() (string, error) {
	out, err := c.run("builder", "prune", arg("force"))
	if err != nil {
		return "", err
	}

	return reclaimedSpace(out), nil
}

// reclaimedSpace returns the size from a prune's "Total reclaimed space: 1GB" or "Total: 1GB" line, or "" without one.
func reclaimedSpace(out string) string {
	for line := range strings.Lines(out) {
		if rest, ok := strings.CutPrefix(line, "Total"); ok {
			_, size, _ := strings.Cut(rest, ":")
			return strings.TrimSpace(size)
		}
	}
	return ""
}
