package microsandbox

import "github.com/dylanvgils/agentic-cli/internal/logging"

// CleanImage removes every ref of image's repository from msb's image store, reporting each one.
func (c *Client) CleanImage(image string) error {
	refs, err := c.ImageRefs()
	if err != nil {
		return err
	}

	repo, _ := SplitRef(image)
	for _, ref := range refs {
		if refRepo, _ := SplitRef(ref); refRepo != repo {
			continue
		}
		if err := c.RemoveImage(ref); err != nil {
			return err
		}
		logging.Detailf("microsandbox %s", ref)
	}
	return nil
}
