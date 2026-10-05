package docker

import "strings"

// CleanImage removes all containers using image (with their anonymous volumes, e.g. the proxy's credentials) and the image itself.
func (c *Client) CleanImage(image string) error {
	if err := c.runIfAny(
		[]string{"ps", arg("all"), arg("quiet"), labelFilter(LabelProject, LabelProjectVal), arg("filter", "ancestor="+image)},
		[]string{"rm", arg("force"), arg("volumes")},
	); err != nil {
		return err
	}

	return c.runIfAny(
		[]string{"images", arg("quiet"), image},
		[]string{"rmi", arg("force")},
	)
}

// CleanBaseImages removes all Docker images whose repository starts with "agentic-base".
func (c *Client) CleanBaseImages() error {
	out, err := c.run("images", arg("format", "{{.Repository}}"))
	if err != nil {
		return err
	}

	var names []string
	for name := range strings.FieldsSeq(out) {
		if strings.HasPrefix(name, baseImage()) {
			names = append(names, name)
		}
	}

	if len(names) == 0 {
		return nil
	}
	_, err = c.run(append([]string{"rmi", arg("force")}, names...)...)
	return err
}

func (c *Client) runIfAny(listArgs, runArgs []string) error {
	out, err := c.run(listArgs...)
	if err != nil {
		return err
	}
	if ids := strings.Fields(out); len(ids) > 0 {
		_, err = c.run(append(runArgs, ids...)...)
		return err
	}
	return nil
}
