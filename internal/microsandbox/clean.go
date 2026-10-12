package microsandbox

import (
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/logging"
)

// CleanImage removes every ref of image's repository from msb's image store, reporting the removed tags in one line.
func (c *Client) CleanImage(image string) error {
	refs, err := c.ImageRefs()
	if err != nil {
		return err
	}

	repo, _ := SplitRef(image)
	var tags []string
	defer func() {
		if len(tags) > 0 {
			logging.Detailf("microsandbox %s (%s)", repo, strings.Join(tags, ", "))
		}
	}()

	for _, ref := range refs {
		refRepo, tag := SplitRef(ref)
		if refRepo != repo {
			continue
		}
		if err := c.RemoveImage(ref); err != nil {
			return err
		}
		tags = append(tags, tag)
	}
	return nil
}
