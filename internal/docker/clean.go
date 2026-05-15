package docker

import "strings"

// CleanImage removes all containers using image and the image itself.
func CleanImage(image string) error {
	if err := runIfAny(
		[]string{"ps", arg("all"), arg("quiet"), arg("filter", "label=project=agentic-cli"), arg("filter", "ancestor="+image)},
		[]string{"rm", arg("force")},
	); err != nil {
		return err
	}

	return runIfAny(
		[]string{"images", arg("quiet"), image},
		[]string{"rmi", arg("force")},
	)
}

// CleanBaseImages removes all Docker images whose repository starts with "agentic-base".
func CleanBaseImages() error {
	out, err := dockerRun("images", arg("format", "{{.Repository}}"))
	if err != nil {
		return err
	}

	var names []string
	for name := range strings.FieldsSeq(out) {
		if strings.HasPrefix(name, "agentic-base") {
			names = append(names, name)
		}
	}

	if len(names) == 0 {
		return nil
	}
	_, err = dockerRun(append([]string{"rmi", arg("force")}, names...)...)
	return err
}

func runIfAny(listArgs, runArgs []string) error {
	out, err := dockerRun(listArgs...)
	if err != nil {
		return err
	}
	if ids := strings.Fields(out); len(ids) > 0 {
		_, err = dockerRun(append(runArgs, ids...)...)
		return err
	}
	return nil
}
