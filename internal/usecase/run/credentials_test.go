package run

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/network/credentials"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_resolveCredentials(t *testing.T) {
	t.Run("no credentials resolves nothing", func(t *testing.T) {
		// Arrange
		layers := []config.RCLayer{{Path: "/example/.agenticrc.toml", RC: &config.AgenticRC{}}}

		// Act
		resolved, err := resolveCredentials(layers, t.TempDir())

		// Assert
		require.NoError(t, err)
		assert.Empty(t, resolved)
	})

	t.Run("unapproved credentials are refused before reading the secret", func(t *testing.T) {
		// Arrange
		layer := credentialLayer(t, "/missing/secret")

		// Act
		_, err := resolveCredentials([]config.RCLayer{layer}, t.TempDir())

		// Assert
		assert.ErrorContains(t, err, "not approved")
	})

	t.Run("approved credentials are resolved", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		secret := filepath.Join(t.TempDir(), "token")
		require.NoError(t, os.WriteFile(secret, []byte("test-secret\n"), 0o600))
		layer := credentialLayer(t, secret)
		approveSettings(t, layer, toolHome)

		// Act
		resolved, err := resolveCredentials([]config.RCLayer{layer}, toolHome)

		// Assert
		require.NoError(t, err)
		require.Len(t, resolved, 1)
		assert.Equal(t, "test-secret", resolved[0].Secret)
		assert.Equal(t, secret, resolved[0].Path)
	})
}

func Test_credentialSetup(t *testing.T) {
	resolved := []credentials.Resolved{{Env: []string{"EXAMPLE_API_KEY"}}}
	proxyInjects := runtimeConfig{Caps: sandbox.Capabilities{Isolation: sandbox.Isolation{CredentialsViaProxy: true}}}
	runtimeInjects := runtimeConfig{}

	t.Run("no credentials is a no-op even with the proxy off", func(t *testing.T) {
		// Act
		err := credentialSetup(runConfig{Runtime: proxyInjects}, mountSet{}, nil)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("credentials without the proxy are an error", func(t *testing.T) {
		// Arrange
		c := runConfig{
			Runtime: proxyInjects,
			Egress:  egressConfig{Credentials: resolved},
		}

		// Act
		err := credentialSetup(c, mountSet{}, nil)

		// Assert
		assert.ErrorContains(t, err, "need the egress proxy")
	})

	t.Run("credentials with the proxy pass and leave the placeholders to the runtime", func(t *testing.T) {
		// Arrange
		c := runConfig{
			Runtime: proxyInjects,
			Egress: egressConfig{
				Mode:        sandbox.EgressMonitor,
				Credentials: resolved,
			},
		}
		mounts := mountSet{toolHome: t.TempDir()}

		// Act
		err := credentialSetup(c, mounts, nil)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("a runtime that injects itself needs no proxy and gets no placeholders, but the same checks", func(t *testing.T) {
		// Arrange
		c := runConfig{
			Runtime: runtimeInjects,
			Egress:  egressConfig{Credentials: resolved},
		}
		mounts := mountSet{toolHome: t.TempDir()}

		// Act
		err := credentialSetup(c, mounts, nil)
		overrideErr := credentialSetup(c, mounts, []string{"EXAMPLE_API_KEY=value"})

		// Assert
		assert.NoError(t, err)
		assert.ErrorContains(t, overrideErr, "set by a proxy credential")
	})
}

func Test_sandboxCredentials(t *testing.T) {
	// Arrange
	resolved := []credentials.Resolved{{
		Hosts:   []string{"api.example.test", "git.example.test"},
		Headers: []string{"Authorization"},
		Env:     []string{"EXAMPLE_TOKEN", "EXAMPLE_KEY"},
		Secret:  "test-secret",
	}}

	// Act
	creds := sandboxCredentials(resolved)

	// Assert
	assert.Equal(t, []sandbox.Credential{{
		Name:    "EXAMPLE_TOKEN",
		Aliases: []string{"EXAMPLE_KEY"},
		Hosts:   []string{"api.example.test", "git.example.test"},
		Headers: []string{"Authorization"},
		Secret:  "test-secret",
	}}, creds)
}

func Test_checkCredentialEnv(t *testing.T) {
	t.Run("accepts names nothing else sets", func(t *testing.T) {
		// Arrange
		in := runConfig{Egress: egressConfig{Credentials: []credentials.Resolved{{Env: []string{"EXAMPLE_TOKEN"}}}}}

		// Act
		err := checkCredentialEnv(in, []string{"OTHER=value"})

		// Assert
		assert.NoError(t, err)
	})

	t.Run("refuses an env entry for a proxy trust var", func(t *testing.T) {
		for _, entry := range []string{"AGENTIC_PROXY_CA=test-ca", "SSL_CERT_FILE=/certs/example.pem", "NODE_EXTRA_CA_CERTS"} {
			t.Run(entry, func(t *testing.T) {
				// Arrange
				in := runConfig{Egress: egressConfig{Credentials: []credentials.Resolved{{Env: []string{"EXAMPLE_TOKEN"}}}}}

				// Act
				err := checkCredentialEnv(in, []string{entry})

				// Assert
				assert.ErrorContains(t, err, "set by agentic when proxy credentials are configured")
			})
		}
	})

	t.Run("refuses an env entry for the same name", func(t *testing.T) {
		for _, entry := range []string{"EXAMPLE_TOKEN=value", "EXAMPLE_TOKEN"} {
			t.Run(entry, func(t *testing.T) {
				// Arrange
				in := runConfig{Egress: egressConfig{Credentials: []credentials.Resolved{{Env: []string{"EXAMPLE_TOKEN"}}}}}

				// Act
				err := checkCredentialEnv(in, []string{entry})

				// Assert
				assert.ErrorContains(t, err, "set by a proxy credential")
			})
		}
	})

	t.Run("refuses names agentic manages", func(t *testing.T) {
		for _, name := range []string{"HTTPS_PROXY", "TOOL_HOME", "AGENTIC_PROXY_CA", "GIT_SSL_CAINFO"} {
			t.Run(name, func(t *testing.T) {
				// Arrange
				in := runConfig{Egress: egressConfig{Credentials: []credentials.Resolved{{Env: []string{name}}}}}

				// Act
				err := checkCredentialEnv(in, nil)

				// Assert
				assert.ErrorContains(t, err, "managed by agentic")
			})
		}
	})

	t.Run("refuses docker names only with dind", func(t *testing.T) {
		// Arrange
		in := runConfig{Egress: egressConfig{Credentials: []credentials.Resolved{{Env: []string{"DOCKER_HOST"}}}}}
		dindIn := in
		dindIn.Dind.Enabled = true

		// Act
		withoutDind := checkCredentialEnv(in, nil)
		withDind := checkCredentialEnv(dindIn, nil)

		// Assert
		assert.NoError(t, withoutDind)
		assert.ErrorContains(t, withDind, "managed by agentic")
	})

	t.Run("refuses integration names only with the integration", func(t *testing.T) {
		// Arrange
		in := runConfig{Egress: egressConfig{Credentials: []credentials.Resolved{{Env: []string{"MAVEN_ARGS"}}}}}
		mavenIn := in
		mavenIn.Egress.Integrations = []string{"maven"}

		// Act
		without := checkCredentialEnv(in, nil)
		with := checkCredentialEnv(mavenIn, nil)

		// Assert
		assert.NoError(t, without)
		assert.ErrorContains(t, with, "managed by agentic")
	})
}

func Test_checkCredentialPaths(t *testing.T) {
	t.Run("secret outside every mount is accepted", func(t *testing.T) {
		// Arrange
		resolved := []credentials.Resolved{{Path: filepath.Join(t.TempDir(), "token")}}
		mounts := mountSet{
			mounts:   parseMounts(t, "", t.TempDir()+":/data", "agentic-cache:/cache"),
			toolHome: t.TempDir(),
		}

		// Act
		err := checkCredentialPaths(resolved, mounts)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("secret inside a bind mount is refused", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		resolved := []credentials.Resolved{{Path: filepath.Join(dir, "nested", "token")}}
		mounts := mountSet{mounts: parseMounts(t, "", dir+":/data:ro"), toolHome: t.TempDir()}

		// Act
		err := checkCredentialPaths(resolved, mounts)

		// Assert
		assert.ErrorContains(t, err, "which the tool container can access")
	})

	t.Run("secret inside a tool home mount is refused", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		resolved := []credentials.Resolved{{Path: filepath.Join(toolHome, "claude", "token")}}
		mounts := mountSet{
			mounts:   parseMounts(t, toolHome, "$TOOL_HOME/claude:$CONTAINER_HOME/.claude"),
			toolHome: toolHome,
		}

		// Act
		err := checkCredentialPaths(resolved, mounts)

		// Assert
		assert.ErrorContains(t, err, "which the tool container can access")
	})

	t.Run("secret inside cwd is refused", func(t *testing.T) {
		// Arrange
		cwd, err := os.Getwd()
		require.NoError(t, err)
		resolved := []credentials.Resolved{{Path: filepath.Join(cwd, "token")}}

		// Act
		err = checkCredentialPaths(resolved, mountSet{})

		// Assert
		assert.ErrorContains(t, err, "which the tool container can access")
	})

	t.Run("symlink pointing into a mount is refused", func(t *testing.T) {
		// Arrange
		if runtime.GOOS == "windows" {
			t.Skip("symlinks need privileges on windows")
		}
		mounted := t.TempDir()
		target := filepath.Join(mounted, "token")
		require.NoError(t, os.WriteFile(target, []byte("test-secret"), 0o600))
		link := filepath.Join(t.TempDir(), "token")
		require.NoError(t, os.Symlink(target, link))
		resolved := []credentials.Resolved{{Path: link}}
		mounts := mountSet{mounts: parseMounts(t, "", mounted+":/data"), toolHome: t.TempDir()}

		// Act
		err := checkCredentialPaths(resolved, mounts)

		// Assert
		assert.ErrorContains(t, err, "which the tool container can access")
	})
}

func Test_checkProxyTrust(t *testing.T) {
	target := Target{ToolName: "claude", ImageName: "agentic-claude"}

	for _, tc := range []struct {
		name    string
		info    *docker.ImageInfo
		wantErr string
	}{
		{
			name: "an image with proxy trust passes",
			info: &docker.ImageInfo{ProxyTrust: true},
		},
		{
			name:    "an image without proxy trust is refused with the rebuild command",
			info:    &docker.ImageInfo{},
			wantErr: "agentic update claude",
		},
		{
			name: "a missing image passes, as it is built fresh",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			err := checkProxyTrust(target, tc.info)

			// Assert
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}

	t.Run("a skipped entrypoint warns instead of refusing", func(t *testing.T) {
		// Arrange
		stderr := iostreamstest.StubErr(t)
		skipped := Target{ToolName: "claude", ImageName: "agentic-claude", SkipEntrypoint: true}

		// Act
		err := checkProxyTrust(skipped, &docker.ImageInfo{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, stderr.String(), "proxy CA is not trusted")
	})
}

func Test_describeInjection(t *testing.T) {
	// Arrange
	creds := []credentials.Resolved{
		{Hosts: []string{"api.example.test", "git.example.test"}, Secret: "test-secret", Source: "/home/user/.secrets/example"},
		{Hosts: []string{"other.example.test"}, Secret: "test-secret", Source: "/home/user/.secrets/other"},
	}

	// Act
	desc := describeInjection(creds)

	// Assert
	assert.Equal(t, "api.example.test, git.example.test (/home/user/.secrets/example); other.example.test (/home/user/.secrets/other)", desc)
	assert.NotContains(t, desc, "test-secret")
}
