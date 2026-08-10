package docker

import "strings"

// ListContexts returns the names of all Docker contexts known to the docker CLI.
func ListContexts() ([]string, error) {
	out, err := dockerRun("context", "ls", arg("format", "{{.Name}}"))
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}
