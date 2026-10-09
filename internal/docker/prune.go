package docker

import "errors"

// PruneImages removes dangling Docker images created by agentic builds.
func (c *Client) PruneImages() error {
	_, err := c.run("image", "prune",
		arg("force"),
		labelFilter(LabelProject, LabelProjectVal))

	return err
}

// PruneBuildCache removes BuildKit cache entries from agentic builds.
func (c *Client) PruneBuildCache() error {
	_, err := c.run("builder", "prune",
		arg("force"),
		labelFilter(LabelProject, LabelProjectVal))

	return err
}

// PruneDangling runs PruneImages and PruneBuildCache, trying both, and returns their combined error.
func (c *Client) PruneDangling() error {
	return errors.Join(c.PruneImages(), c.PruneBuildCache())
}
