package docker

import "strings"

// dockerContext is the Docker context (docker --context) prepended to every
// docker invocation, set once per process via SetContext. Empty means the
// docker CLI's own active context is used, unchanged.
var dockerContext string

// SetContext sets the Docker context used for all subsequent docker invocations.
func SetContext(ctx string) {
	dockerContext = ctx
}

// Context returns the Docker context currently set via SetContext.
func Context() string {
	return dockerContext
}

// withContext prepends "--context <name>" to args when a context is set.
func withContext(args []string) []string {
	if dockerContext == "" {
		return args
	}
	return append([]string{"--context", dockerContext}, args...)
}

// ListContexts returns the names of all Docker contexts known to the docker CLI.
func ListContexts() ([]string, error) {
	out, err := dockerRun("context", "ls", arg("format", "{{.Name}}"))
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}
