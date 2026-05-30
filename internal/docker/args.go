package docker

import (
	"os"
	"strings"
)

// forwardEnvArg builds --env=KEY=val flags for each key set in the host environment.
func forwardEnvArg(keys ...string) []string {
	var args []string
	for _, key := range keys {
		if val := os.Getenv(key); val != "" {
			args = append(args, arg("env", key+"="+val))
		}
	}
	return args
}

// arg builds a --name or --name=value Docker flag.
// Panics if name is empty, starts with '-', or more than one value is given (programmer error).
func arg(name string, value ...string) string {
	if name == "" {
		panic("docker: arg name must not be empty")
	}

	if strings.HasPrefix(name, "-") {
		panic("docker: arg name must not start with '-', got: " + name)
	}

	if len(value) > 1 {
		panic("docker: arg accepts at most one value")
	}

	if len(value) == 0 {
		return "--" + name
	}

	return "--" + name + "=" + value[0]
}
