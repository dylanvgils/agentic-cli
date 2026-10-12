package mount

import (
	"fmt"
	"strings"
)

// ParseSecretSpec splits a secret spec into its host path and the path inside the sandbox, which defaults to /run/secrets/<name>.
func ParseSecretSpec(spec string) (host, target string, err error) {
	name, rest, ok := strings.Cut(spec, ":")
	if !ok {
		return "", "", fmt.Errorf("invalid secret %q: expected name:/path[:/container/path]", spec)
	}

	host, target, found := SplitHostContainer(rest)
	if !found {
		return host, "/run/secrets/" + name, nil
	}
	if target == "" {
		return "", "", fmt.Errorf("invalid secret %q: empty container path", spec)
	}
	return host, target, nil
}
