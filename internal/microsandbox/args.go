package microsandbox

import "strings"

// arg builds a --name or --name=value msb flag; panics on a malformed name or more than one value.
func arg(name string, value ...string) string {
	if name == "" {
		panic("microsandbox: arg name must not be empty")
	}

	if strings.HasPrefix(name, "-") {
		panic("microsandbox: arg name must not start with '-', got: " + name)
	}

	if len(value) > 1 {
		panic("microsandbox: arg accepts at most one value")
	}

	if len(value) == 0 {
		return "--" + name
	}

	return "--" + name + "=" + value[0]
}
