// Package credentials resolves [[run.proxy.credentials]] entries into header rules for the egress proxy.
package credentials

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
)

// Placeholder is the value a credential's env vars get in the tool container instead of the secret.
const Placeholder = "agentic-proxy-managed"

// validEnvName matches a portable environment variable name.
var validEnvName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Resolved is one config entry ready for the proxy.
type Resolved struct {
	// Proxy holds the hosts and formatted header values; it contains the secret, so never log or print it
	Proxy []proxy.Credential
	// Env lists the tool env vars to set to Placeholder
	Env []string
	// Source describes where the secret came from, for display
	Source string
}

// Hosts returns every host the entry injects into.
func (r Resolved) Hosts() []string {
	var hosts []string
	for _, cred := range r.Proxy {
		hosts = append(hosts, cred.Hosts...)
	}
	return hosts
}

// Resolve reads each entry's secret and formats its header values, failing on invalid entries or a host claimed twice.
func Resolve(entries []config.RCCredential) ([]Resolved, error) {
	var resolved []Resolved
	var all []proxy.Credential

	for i, entry := range entries {
		r, err := resolveEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("credential %d (%s): %w", i+1, describe(entry), err)
		}

		resolved = append(resolved, r)
		all = append(all, r.Proxy...)
	}

	if err := proxy.ValidateCredentials(all); err != nil {
		return nil, fmt.Errorf("credentials: %w", err)
	}
	return resolved, nil
}

// resolveEntry expands entry and fills in its secret.
func resolveEntry(entry config.RCCredential) (Resolved, error) {
	targets, env, err := expand(entry)
	if err != nil {
		return Resolved{}, err
	}

	for _, name := range env {
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

	creds := make([]proxy.Credential, 0, len(targets))
	for _, target := range targets {
		cred := proxy.Credential{
			Hosts: target.hosts,
			Rules: []proxy.InjectRule{{Header: target.header, Value: target.value(secret)}},
		}
		if err := proxy.ValidateCredential(cred); err != nil {
			return Resolved{}, err
		}
		creds = append(creds, cred)
	}

	return Resolved{Proxy: creds, Env: env, Source: source.String()}, nil
}

// expand returns the entry's targets and env, from its preset or its own hosts and header; an explicit env replaces a preset's.
func expand(entry config.RCCredential) ([]target, []string, error) {
	if entry.Preset == "" {
		value, err := formatter(entry.Format)
		if err != nil {
			return nil, nil, err
		}
		return []target{{hosts: entry.Hosts, header: entry.Header, value: value}}, entry.Env, nil
	}

	preset, ok := presets[entry.Preset]
	if !ok {
		return nil, nil, fmt.Errorf("unknown preset %q", entry.Preset)
	}

	env := preset.env
	if len(entry.Env) > 0 {
		env = entry.Env
	}
	return preset.targets, env, nil
}

// formatter returns a func placing the secret at the single %s in format; empty means the bare secret.
func formatter(format string) (func(string) string, error) {
	if format == "" {
		return bare, nil
	}

	before, after, ok := strings.Cut(format, "%s")
	if !ok || strings.Contains(before, "%") || strings.Contains(after, "%") {
		return nil, fmt.Errorf("format %q must contain exactly one %%s and no other %%", format)
	}
	return func(secret string) string { return before + secret + after }, nil
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
