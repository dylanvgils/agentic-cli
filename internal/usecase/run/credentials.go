package run

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/credentials"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// checkProxyTrust makes sure the tool can trust the proxy CA its entrypoint adds: it warns when the entrypoint is
// skipped and refuses an image whose entrypoint predates it, since TLS to credential hosts would fail either way.
func (s *Service) checkProxyTrust(target Target) error {
	if target.SkipEntrypoint {
		logging.Warnf("skipping the entrypoint: TLS to proxy credential hosts will fail, as the proxy CA is not trusted")
		return nil
	}

	info, err := s.docker.InspectImage(target.ImageName)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", target.ImageName, err)
	}
	if info == nil || info.ProxyTrust {
		return nil
	}
	return fmt.Errorf("%s can't trust the proxy CA, so TLS to credential hosts would fail; rebuild it with \"agentic update %s\"", target.ImageName, target.ToolName)
}

// resolveCredentials reads the secrets of every credential the layers declare, refusing to run while any layer's entries are unapproved.
func resolveCredentials(layers []config.RCLayer, toolHome string) ([]credentials.Resolved, error) {
	rc, err := config.Merge(layers)
	if err != nil {
		return nil, err
	}
	if len(rc.Run.Proxy.Credentials) == 0 {
		return nil, nil
	}

	// Checked here too, so no caller can read a secret the user hasn't approved
	state, err := config.LoadState(toolHome)
	if err != nil {
		return nil, fmt.Errorf("load credential approvals: %w", err)
	}
	for _, layer := range layers {
		if slices.ContainsFunc(state.ChangedSettings(layer), func(s config.GuardedSetting) bool { return s.Key == config.CredentialsSetting }) {
			return nil, fmt.Errorf("proxy credentials in %s are new or changed and not approved", layer.Path)
		}
	}

	return credentials.Resolve(rc.Run.Proxy.Credentials)
}

// credentialSetup checks in.Credentials against the run and returns the placeholder env for the tool; a no-op without credentials.
func credentialSetup(in Input, mounts mountSet, env []string) ([]string, error) {
	if len(in.Credentials) == 0 {
		return nil, nil
	}
	if !in.ProxyMode.Enabled() {
		return nil, fmt.Errorf("proxy credentials need the egress proxy, which is disabled")
	}
	if i := slices.IndexFunc(env, func(entry string) bool { return isProxyTrustEnvName(envKey(entry)) }); i >= 0 {
		return nil, fmt.Errorf("--env: %q is set by agentic when proxy credentials are configured", envKey(env[i]))
	}

	if err := checkCredentialPaths(in.Credentials, mounts); err != nil {
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
			if docker.IsReservedEnvName(name, true) || isProxyTrustEnvName(name) || (dindEnabled && docker.IsReservedDindEnvName(name)) {
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
func checkCredentialPaths(resolved []credentials.Resolved, mounts mountSet) error {
	roots := mounts.hostPaths()
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

// isProxyTrustEnvName reports whether name carries the proxy CA or is pointed at its bundle by the tool's entrypoint.
func isProxyTrustEnvName(name string) bool {
	return name == tools.ProxyCAEnvName || slices.Contains(tools.ProxyTrustEnvNames, name)
}

// describeInjection lists each credential's hosts and secret source for the per-run notice; never the secret itself.
func describeInjection(creds []credentials.Resolved) string {
	parts := make([]string, 0, len(creds))
	for _, cred := range creds {
		parts = append(parts, fmt.Sprintf("%s (%s)", strings.Join(cred.Hosts(), ", "), cred.Source))
	}
	return strings.Join(parts, "; ")
}
