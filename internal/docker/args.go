package docker

import "strings"

// arg builds a --name=value Docker flag.
// Panics if name is empty or starts with '-' (programmer error).
func arg(name, value string) string {
	if name == "" {
		panic("docker: arg name must not be empty")
	}

	if strings.HasPrefix(name, "-") {
		panic("docker: arg name must not start with '-', got: " + name)
	}

	return "--" + name + "=" + value
}
