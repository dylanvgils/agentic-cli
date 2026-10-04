package run

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/credentials"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
)

// ResolveCredentials reads the secrets of every credential the layers declare, refusing to run while any layer's entries are unapproved.
func ResolveCredentials(layers []config.RCLayer, toolHome string) ([]credentials.Resolved, error) {
	rc, err := config.Merge(layers)
	if err != nil {
		return nil, err
	}
	if len(rc.Run.Proxy.Credentials) == 0 {
		return nil, nil
	}

	// Checked here too, so no entrypoint can read a secret the user hasn't approved
	cfg, err := config.LoadConfig(toolHome)
	if err != nil {
		return nil, fmt.Errorf("load credential approvals: %w", err)
	}
	if pending := cfg.PendingCredentials(layers); len(pending) > 0 {
		return nil, fmt.Errorf("proxy credentials in %s are new or changed and not approved", pending[0].Path)
	}

	return credentials.Resolve(rc.Run.Proxy.Credentials)
}

// credentialSetup checks in.Credentials against the run and returns the placeholder env for the tool; a no-op without credentials.
func credentialSetup(in Input, volumes, secrets, env []string, containerHome string) ([]string, error) {
	if len(in.Credentials) == 0 {
		return nil, nil
	}
	if !in.ProxyMode.Enabled() {
		return nil, fmt.Errorf("proxy credentials need the egress proxy, which is disabled")
	}
	if slices.ContainsFunc(env, func(entry string) bool { return envKey(entry) == docker.ProxyCAEnvName }) {
		return nil, fmt.Errorf("--env: %q is set by agentic when proxy credentials are configured", docker.ProxyCAEnvName)
	}

	if err := checkCredentialPaths(in.Credentials, volumes, secrets, in.ToolHome, containerHome); err != nil {
		return nil, err
	}
	return credentialEnv(in.Credentials, env, in.DindEnabled)
}

// proxyCredentials flattens resolved into the list the proxy injects.
func proxyCredentials(resolved []credentials.Resolved) []proxy.Credential {
	var creds []proxy.Credential
	for _, r := range resolved {
		creds = append(creds, r.Proxy...)
	}
	return creds
}

// credentialEnv returns NAME=Placeholder for each credential env var, refusing names agentic manages or env sets,
// since a forwarded host value would leak the real secret.
func credentialEnv(resolved []credentials.Resolved, env []string, dindEnabled bool) ([]string, error) {
	var result []string
	for _, r := range resolved {
		for _, name := range r.Env {
			if docker.IsReservedEnvName(name, true) || name == docker.ProxyCAEnvName || (dindEnabled && docker.IsReservedDindEnvName(name)) {
				return nil, fmt.Errorf("credential env %q is managed by agentic", name)
			}
			if slices.ContainsFunc(env, func(entry string) bool { return envKey(entry) == name }) {
				return nil, fmt.Errorf("--env: %q is set by a proxy credential and cannot be overridden", name)
			}
			result = append(result, name+"="+credentials.Placeholder)
		}
	}
	return result, nil
}

// checkCredentialPaths refuses secrets inside cwd or a host path mounted into the tool container.
// Approval covers a secret's path, not its content, so the agent must not be able to read or replace the file.
func checkCredentialPaths(resolved []credentials.Resolved, volumes, secrets []string, toolHome, containerHome string) error {
	roots := mountedHostPaths(volumes, secrets, toolHome, containerHome)
	if cwd, err := os.Getwd(); err == nil {
		roots = append(roots, cwd)
	}

	for _, r := range resolved {
		if r.Path == "" {
			continue
		}
		if root, ok := findRoot(r.Path, roots); ok {
			return fmt.Errorf("credential secret %s is inside %s, which the tool container can access", r.Path, root)
		}
	}

	return nil
}

// envKey returns the name part of a KEY=VALUE or bare KEY entry.
func envKey(entry string) string {
	key, _, _ := strings.Cut(entry, "=")
	return key
}
