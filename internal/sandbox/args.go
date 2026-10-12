// Package sandbox holds what the container and microVM runtimes share to run a tool.
package sandbox

import "strings"

// Arg builds a --name or --name=value CLI flag; panics on a malformed name or more than one value.
func Arg(name string, value ...string) string {
	if name == "" {
		panic("sandbox: arg name must not be empty")
	}

	if strings.HasPrefix(name, "-") {
		panic("sandbox: arg name must not start with '-', got: " + name)
	}

	if len(value) > 1 {
		panic("sandbox: arg accepts at most one value")
	}

	if len(value) == 0 {
		return "--" + name
	}

	return "--" + name + "=" + value[0]
}

// ShellJoin renders args as a shell command line, single-quoting any arg that needs it.
func ShellJoin(args []string) string {
	parts := make([]string, len(args))
	for i, arg := range args {
		if strings.ContainsAny(arg, " \t$'") {
			parts[i] = "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
		} else {
			parts[i] = arg
		}
	}
	return strings.Join(parts, " ")
}
