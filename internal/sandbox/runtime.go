package sandbox

import "strings"

// Runtimes a tool can run on; microsandbox is experimental.
const (
	RuntimeDocker       Runtime = "docker"
	RuntimeMicrosandbox Runtime = "microsandbox"
)

// Runtimes lists every runtime, in the order help and errors name them.
var Runtimes = []Runtime{
	RuntimeDocker,
	RuntimeMicrosandbox,
}

// Runtime is where a tool runs.
type Runtime string

// RuntimeNames returns every runtime's name joined with "or", for help and error text.
func RuntimeNames() string {
	names := make([]string, len(Runtimes))
	for i, r := range Runtimes {
		names[i] = string(r)
	}
	return strings.Join(names, " or ")
}
