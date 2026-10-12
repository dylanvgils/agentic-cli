package sandbox

import (
	"fmt"
	"strings"
)

// Credential is a secret the runtime substitutes for a placeholder on requests to Hosts; Secret is never printed.
type Credential struct {
	Name    string   // env var carrying the placeholder
	Aliases []string // further env vars carrying the same placeholder
	Hosts   []string
	Headers []string // headers the secret may be substituted in
	Secret  string
}

// String hides Secret, so printing a credential never leaks it.
func (c Credential) String() string {
	return fmt.Sprintf("%s@%s: [redacted]", c.Name, strings.Join(c.Hosts, ","))
}
