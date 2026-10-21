package microsandbox

import (
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
)

// CleanImage removes every ref of image's repository from msb's image store, reporting the removed tags in one line.
func (c *Client) CleanImage(image string) error {
	refs, err := c.ImageRefs()
	if err != nil {
		return err
	}

	repo, _ := SplitRef(image)
	matching, tags := repoRefs(refs, repo)
	for _, ref := range matching {
		if err := c.RemoveImage(ref); err != nil {
			return err
		}
	}

	if len(tags) > 0 {
		logging.Detailf("microsandbox %s (%s)", repo, strings.Join(tags, ", "))
	}
	return nil
}

// repoRefs returns the refs of repo among refs, and their tags.
func repoRefs(refs []string, repo string) (matching, tags []string) {
	for _, ref := range refs {
		if refRepo, tag := SplitRef(ref); refRepo == repo {
			matching = append(matching, ref)
			tags = append(tags, tag)
		}
	}
	return matching, tags
}
