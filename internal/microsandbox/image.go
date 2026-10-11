package microsandbox

import (
	"fmt"
	"io"
	"strings"
)

// LoadImage loads a `docker save` tar archive read from r into msb's image store, adding tag as a local reference.
func (c *Client) LoadImage(r io.Reader, tag string) error {
	if _, err := c.runner.RunWithStdin(r, "load", arg("tag", tag)); err != nil {
		return fmt.Errorf("load image into msb: %w", err)
	}
	return nil
}

// ImageRefs lists the references of the images in msb's image store.
func (c *Client) ImageRefs() ([]string, error) {
	out, err := c.runner.Run("images", arg("quiet"))
	if err != nil {
		return nil, fmt.Errorf("list msb images: %w", err)
	}

	var refs []string
	for line := range strings.Lines(out) {
		if ref := strings.TrimSpace(line); ref != "" {
			refs = append(refs, ref)
		}
	}
	return refs, nil
}

// RemoveImage removes ref from msb's image store; msb keeps the image while a sandbox uses it.
func (c *Client) RemoveImage(ref string) error {
	if _, err := c.runner.Run("rmi", ref); err != nil {
		return fmt.Errorf("remove msb image %s: %w", ref, err)
	}
	return nil
}
