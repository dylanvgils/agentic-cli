package docker

import "strings"

// PruneImages removes dangling agentic images and returns the space reclaimed.
func (c *Client) PruneImages() (string, error) {
	out, err := c.run("image", "prune",
		arg("force"),
		labelFilter(LabelProject, LabelProjectVal))
	if err != nil {
		return "", err
	}

	return reclaimedSpace(out), nil
}

// PruneBuildCache removes all unused build cache, not just agentic's, and returns the space reclaimed.
func (c *Client) PruneBuildCache() (string, error) {
	out, err := c.run("builder", "prune", arg("force"))
	if err != nil {
		return "", err
	}

	return reclaimedSpace(out), nil
}

// reclaimedSpace returns the size on a prune's "Total" line, or "".
func reclaimedSpace(out string) string {
	for line := range strings.Lines(out) {
		if rest, ok := strings.CutPrefix(line, "Total"); ok {
			_, size, _ := strings.Cut(rest, ":")
			return strings.TrimSpace(size)
		}
	}
	return ""
}
