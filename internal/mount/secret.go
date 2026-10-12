package mount

import (
	"fmt"
	"strings"
)

// ParseSecretSpec splits a "name:/path[:/container/path]" secret into its host and container paths; the container path defaults to /run/secrets/<name>.
func ParseSecretSpec(spec string) (host, container string, err error) {
	name, rest, ok := strings.Cut(spec, ":")
	if !ok {
		return "", "", fmt.Errorf("invalid secret %q: expected name:/path[:/container/path]", spec)
	}

	host, container, found := SplitHostContainer(rest)
	if !found {
		return host, "/run/secrets/" + name, nil
	}
	if container == "" {
		return "", "", fmt.Errorf("invalid secret %q: empty container path", spec)
	}
	return host, container, nil
}
