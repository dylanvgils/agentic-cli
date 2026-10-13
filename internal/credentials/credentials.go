// Package credentials resolves [[run.proxy.credentials]] entries into secrets a runtime swaps in for the placeholders the tool sends.
package credentials

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
)

// placeholderPrefix starts every placeholder, so a tool's env shows the value is agentic's stand-in.
const placeholderPrefix = "agentic-"

// validEnvName matches a portable environment variable name.
var validEnvName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Resolved is one config entry with its secret read.
type Resolved struct {
	Hosts   []string
	Headers []string
	// Env lists the tool env vars that carry the placeholder
	Env []string
	// Placeholder is a random stand-in for Secret, new each run, for runtimes that use agentic's own (the Docker proxy)
	Placeholder string
	// Secret is the raw value; never log or print it
	Secret string
	// Source describes where the secret came from, for display
	Source string
	// Path is the host file the secret was read from, empty for non-file sources
	Path string
	// Git is how git authenticates to a host of the entry over HTTPS; nil when the entry doesn't cover git
	Git *GitAuth
}

// GitAuth is the user git sends with the placeholder as password to Host, which only asks after a 401 challenge.
type GitAuth struct {
	Host string
	User string
}

// ProxyCredential returns r as the agentic proxy's credential.
func (r Resolved) ProxyCredential() proxy.Credential {
	return proxy.Credential{Hosts: r.Hosts, Headers: r.Headers, Placeholder: r.Placeholder, Secret: r.Secret}
}

// Resolve reads each entry's secret and gives it a placeholder, failing on invalid entries, a host claimed twice or an
// entry without env, which no tool could send the placeholder in.
func Resolve(entries []config.RCCredential) ([]Resolved, error) {
	var resolved []Resolved
	var all []proxy.Credential
	for i, entry := range entries {
		r, err := resolveEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("credential %d (%s): %w", i+1, describe(entry), err)
		}
		resolved = append(resolved, r)
		all = append(all, r.ProxyCredential())
	}

	if err := proxy.ValidateCredentials(all); err != nil {
		return nil, fmt.Errorf("credentials: %w", err)
	}
	return resolved, nil
}

// Hosts returns the hosts entries cover without reading any secret; an entry with an unknown preset is skipped.
func Hosts(entries []config.RCCredential) []string {
	var hosts []string
	for _, entry := range entries {
		if e, err := expand(entry); err == nil {
			hosts = append(hosts, e.hosts...)
		}
	}
	return hosts
}

// resolveEntry expands entry, reads its secret and draws its placeholder.
func resolveEntry(entry config.RCCredential) (Resolved, error) {
	e, err := expand(entry)
	if err != nil {
		return Resolved{}, err
	}

	if len(e.env) == 0 {
		return Resolved{}, fmt.Errorf("needs env, as the tool must send the placeholder")
	}
	for _, name := range e.env {
		if !validEnvName.MatchString(name) {
			return Resolved{}, fmt.Errorf("invalid env name %q", name)
		}
	}

	source, err := ParseSource(entry.Secret)
	if err != nil {
		return Resolved{}, err
	}
	secret, err := readSecret(source)
	if err != nil {
		return Resolved{}, err
	}

	placeholder, err := newPlaceholder()
	if err != nil {
		return Resolved{}, err
	}

	var path string
	if file, ok := source.(fileSource); ok {
		path = file.path
	}

	r := Resolved{
		Hosts:       e.hosts,
		Headers:     e.headers,
		Env:         e.env,
		Placeholder: placeholder,
		Secret:      secret,
		Source:      source.String(),
		Path:        path,
		Git:         e.git,
	}
	if err := proxy.ValidateCredential(r.ProxyCredential()); err != nil {
		return Resolved{}, err
	}
	return r, nil
}

// expand returns the entry's hosts, headers, env and git auth, from its preset or its own fields; an explicit env
// replaces a preset's.
func expand(entry config.RCCredential) (preset, error) {
	if entry.Preset == "" {
		return preset{hosts: entry.Hosts, headers: []string{entry.Header}, env: entry.Env}, nil
	}

	p, ok := presets[entry.Preset]
	if !ok {
		return preset{}, fmt.Errorf("unknown preset %q", entry.Preset)
	}
	if len(entry.Env) > 0 {
		p.env = entry.Env
	}
	return p, nil
}

// newPlaceholder returns a random placeholder no real header value contains by chance.
func newPlaceholder() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate placeholder: %w", err)
	}
	return placeholderPrefix + hex.EncodeToString(b), nil
}

// readSecret resolves source and trims surrounding whitespace, rejecting empty or multi-line values.
func readSecret(source SecretSource) (string, error) {
	data, err := source.Resolve()
	if err != nil {
		return "", err
	}

	secret := strings.TrimSpace(string(data))
	if secret == "" {
		return "", fmt.Errorf("secret %s is empty", source)
	}

	// A single-line token is all a header can carry; this also refuses key files and other multi-line secrets
	for _, c := range secret {
		if c < 0x20 || c == 0x7f {
			return "", fmt.Errorf("secret %s contains line breaks or control characters", source)
		}
	}
	return secret, nil
}

// describe names an entry in errors by its preset or hosts.
func describe(entry config.RCCredential) string {
	if entry.Preset != "" {
		return "preset " + entry.Preset
	}
	return strings.Join(entry.Hosts, ", ")
}
