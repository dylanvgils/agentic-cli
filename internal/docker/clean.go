package docker

import "strings"

// CleanImage removes all containers using image and the image itself.
func CleanImage(image string) error {
	args := []string{
		"ps",
		arg("all"),
		arg("quiet"),
		arg("filter", "label=project=agentic-cli"),
		arg("filter", "ancestor="+image),
	}
	out, err := dockerRun(args...)
	if err != nil {
		return err
	}

	if ids := strings.Fields(out); len(ids) > 0 {
		args = append([]string{"rm", arg("force")}, ids...)
		if _, err := dockerRun(args...); err != nil {
			return err
		}
	}

	args = []string{
		"images",
		arg("quiet"),
		image,
	}
	out, err = dockerRun(args...)
	if err != nil {
		return err
	}

	if ids := strings.Fields(out); len(ids) > 0 {
		args = append([]string{"rmi", arg("force")}, ids...)
		if _, err := dockerRun(args...); err != nil {
			return err
		}
	}

	return nil
}

// CleanBaseImages removes all Docker images whose repository starts with "agentic-base".
func CleanBaseImages() error {
	args := []string{
		"images",
		arg("format", "{{.Repository}}"),
	}
	out, err := dockerRun(args...)
	if err != nil {
		return err
	}

	var names []string
	for name := range strings.FieldsSeq(out) {
		if strings.HasPrefix(name, "agentic-base") {
			names = append(names, name)
		}
	}

	if len(names) > 0 {
		args = append([]string{"rmi", arg("force")}, names...)
		if _, err := dockerRun(args...); err != nil {
			return err
		}
	}

	return nil
}
