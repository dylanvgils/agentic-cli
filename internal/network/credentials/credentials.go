// Package credentials resolves [[run.proxy.credentials]] entries into secrets a runtime swaps in for the placeholders the tool sends.
package credentials

import (
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// Resolved is one config entry with its secret read.
type Resolved struct {
	Hosts   []string
	Headers []string
	// Env lists the tool env vars that carry the placeholder
	Env []string
	// Secret is the raw value; never log or print it
	Secret string
	// Source describes where the secret came from, for display
	Source string
	// Path is the host file the secret was read from, empty for non-file sources
	Path string
}

// Resolve reads each entry's secret, failing on invalid entries, a host claimed twice or an entry without env, which no
// tool could send a placeholder in.
func Resolve(entries []config.RCCredential) ([]Resolved, error) {
	var resolved []Resolved
	var all []proxy.Credential
	for i, entry := range entries {
		r, err := resolveEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("credential %d (%s): %w", i+1, describe(entry), err)
		}
		resolved = append(resolved, r)
		all = append(all, validationCredential(r, i))
	}

	if err := proxy.ValidateCredentials(all); err != nil {
		return nil, fmt.Errorf("credentials: %w", err)
	}
	return resolved, nil
}

// Check validates entries without reading any secret: known presets and env names a tool can send the placeholder in. It
// runs before any prompt; Resolve repeats it.
func Check(entries []config.RCCredential) error {
	for i, entry := range entries {
		if _, err := checkEntry(entry); err != nil {
			return fmt.Errorf("credential %d (%s): %w", i+1, describe(entry), err)
		}
	}
	return nil
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

// checkEntry expands entry and checks it has valid env names, which no tool could send the placeholder without.
func checkEntry(entry config.RCCredential) (preset, error) {
	e, err := expand(entry)
	if err != nil {
		return preset{}, err
	}

	if len(e.env) == 0 {
		return preset{}, fmt.Errorf("needs env, as the tool must send the placeholder")
	}
	for _, name := range e.env {
		if !sandbox.IsPortableEnvName(name) {
			return preset{}, fmt.Errorf("invalid env name %q", name)
		}
	}
	return e, nil
}

// resolveEntry checks entry and reads its secret.
func resolveEntry(entry config.RCCredential) (Resolved, error) {
	e, err := checkEntry(entry)
	if err != nil {
		return Resolved{}, err
	}

	source, err := ParseSource(entry.Secret)
	if err != nil {
		return Resolved{}, err
	}
	secret, err := readSecret(source)
	if err != nil {
		return Resolved{}, err
	}

	var path string
	if file, ok := source.(fileSource); ok {
		path = file.path
	}

	r := Resolved{
		Hosts:   e.hosts,
		Headers: e.headers,
		Env:     e.env,
		Secret:  secret,
		Source:  source.String(),
		Path:    path,
	}
	if err := proxy.ValidateCredential(validationCredential(r, 0)); err != nil {
		return Resolved{}, err
	}
	return r, nil
}

// expand returns the entry's hosts, headers and env, from its preset or its own fields; an explicit env
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

// validationCredential returns r as a proxy credential with the i-th stand-in placeholder, so Resolve can check r against
// the proxy's rules before the runtime draws the real placeholders.
func validationCredential(r Resolved, i int) proxy.Credential {
	return proxy.Credential{Hosts: r.Hosts, Headers: r.Headers, Placeholder: fmt.Sprintf("agentic-validate-%08d", i), Secret: r.Secret}
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
