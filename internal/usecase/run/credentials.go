package run

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/credentials"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// checkProxyTrust makes sure the tool can trust the proxy CA its entrypoint adds: it warns when the entrypoint is
// skipped and refuses an image whose entrypoint predates it, since TLS to the hosts the proxy decrypts would fail either
// way.
func (s *Service) checkProxyTrust(target Target) error {
	if target.SkipEntrypoint {
		logging.Warnf("skipping the entrypoint: TLS to proxy credential and path rule hosts will fail, as the proxy CA is not trusted")
		return nil
	}

	info, err := s.docker.InspectImage(target.ImageName)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", target.ImageName, err)
	}
	if info == nil || info.ProxyTrust {
		return nil
	}
	return fmt.Errorf("%s can't trust the proxy CA, so TLS to credential and path rule hosts would fail; rebuild it with \"agentic update %s\"", target.ImageName, target.ToolName)
}

// gitConfigEnvPrefix starts the env vars git reads config from, which the git credential helper is set through.
const gitConfigEnvPrefix = "GIT_CONFIG_"

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

// credentialSetup checks in.Credentials against the run and returns the tool's git helper env; the runtime sets the
// placeholders itself. A no-op without credentials.
func credentialSetup(in Input, caps sandbox.Capabilities, mounts mountSet, env []string) ([]string, error) {
	if len(in.Credentials) == 0 {
		return nil, nil
	}
	if caps.CredentialsViaProxy && !in.ProxyMode.Enabled() {
		return nil, fmt.Errorf("proxy credentials need the egress proxy, which is disabled")
	}

	if err := checkCredentialPaths(in.Credentials, mounts); err != nil {
		return nil, err
	}
	if err := checkCredentialEnv(in.Credentials, env, in.DindEnabled); err != nil {
		return nil, err
	}

	return gitHelperEnv(in.Credentials), nil
}

// sandboxCredentials maps each resolved entry to a sandbox.Credential named after its first env var.
func sandboxCredentials(resolved []credentials.Resolved) []sandbox.Credential {
	creds := make([]sandbox.Credential, 0, len(resolved))
	for _, r := range resolved {
		creds = append(creds, sandbox.Credential{
			Name:    r.Env[0],
			Aliases: r.Env[1:],
			Hosts:   r.Hosts,
			Headers: r.Headers,
			Secret:  r.Secret,
		})
	}
	return creds
}

// gitHelperEnv sets a git credential helper for each credential covering git, answering its user and the placeholder in its
// first env var; git reads it from env, as the tool's home is read-only. Nil when no credential covers git.
func gitHelperEnv(resolved []credentials.Resolved) []string {
	var helpers []string
	for _, r := range resolved {
		if r.Git == nil {
			continue
		}

		i := len(helpers) / 2
		helper := fmt.Sprintf(`!f() { echo username=%s; echo "password=$%s"; }; f`, r.Git.User, r.Env[0])
		helpers = append(helpers,
			fmt.Sprintf("%sKEY_%d=credential.https://%s.helper", gitConfigEnvPrefix, i, r.Git.Host),
			fmt.Sprintf("%sVALUE_%d=%s", gitConfigEnvPrefix, i, helper),
		)
	}

	if len(helpers) == 0 {
		return nil
	}
	return append([]string{fmt.Sprintf("%sCOUNT=%d", gitConfigEnvPrefix, len(helpers)/2)}, helpers...)
}

// checkCredentialEnv refuses env entries for the proxy trust vars, and credential env names agentic manages or env sets,
// since a forwarded host value would leak the real secret.
func checkCredentialEnv(resolved []credentials.Resolved, env []string, dindEnabled bool) error {
	if i := slices.IndexFunc(env, func(entry string) bool { return isProxyTrustEnvName(envKey(entry)) }); i >= 0 {
		return fmt.Errorf("--env: %q is set by agentic when proxy credentials are configured", envKey(env[i]))
	}

	coversGit := slices.ContainsFunc(resolved, func(r credentials.Resolved) bool { return r.Git != nil })
	if i := slices.IndexFunc(env, func(entry string) bool { return strings.HasPrefix(envKey(entry), gitConfigEnvPrefix) }); coversGit && i >= 0 {
		return fmt.Errorf("--env: %q is set by agentic for the git credential helper", envKey(env[i]))
	}

	for _, r := range resolved {
		for _, name := range r.Env {
			if sandbox.IsReservedEnvName(name, true, dindEnabled) || isProxyTrustEnvName(name) {
				return fmt.Errorf("credential env %q is managed by agentic", name)
			}
			if slices.ContainsFunc(env, func(entry string) bool { return envKey(entry) == name }) {
				return fmt.Errorf("--env: %q is set by a proxy credential and cannot be overridden", name)
			}
		}
	}
	return nil
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
		parts = append(parts, fmt.Sprintf("%s (%s)", strings.Join(cred.Hosts, ", "), cred.Source))
	}
	return strings.Join(parts, "; ")
}
