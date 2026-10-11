package microsandbox

import (
	"fmt"
	"io"
)

// LoadImage loads a `docker save` tar archive read from r into msb's image store.
func (c *Client) LoadImage(r io.Reader) error {
	if _, err := c.runner.Run(r, "load"); err != nil {
		return fmt.Errorf("load image into msb: %w", err)
	}
	return nil
}
