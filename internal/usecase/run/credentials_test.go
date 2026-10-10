package run

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/credentials"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
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
		assert.Equal(t, "test-secret", resolved[0].Proxy[0].Rules[0].Value)
		assert.Equal(t, secret, resolved[0].Path)
	})
}

func Test_credentialSetup(t *testing.T) {
	resolved := []credentials.Resolved{{Env: []string{"EXAMPLE_API_KEY"}}}

	t.Run("no credentials is a no-op even with the proxy off", func(t *testing.T) {
		// Act
		env, err := credentialSetup(Input{}, mountSet{}, nil)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, env)
	})

	t.Run("credentials without the proxy are an error", func(t *testing.T) {
		// Arrange
		in := Input{Credentials: resolved}

		// Act
		_, err := credentialSetup(in, mountSet{}, nil)

		// Assert
		assert.ErrorContains(t, err, "need the egress proxy")
	})

	t.Run("refuses an env entry for a proxy trust var", func(t *testing.T) {
		for _, entry := range []string{"AGENTIC_PROXY_CA=test-ca", "SSL_CERT_FILE=/certs/example.pem", "NODE_EXTRA_CA_CERTS"} {
			t.Run(entry, func(t *testing.T) {
				// Arrange
				in := Input{ProxyMode: docker.ProxyEnforce, Credentials: resolved}

				// Act
				_, err := credentialSetup(in, mountSet{}, []string{entry})

				// Assert
				assert.ErrorContains(t, err, "set by agentic when proxy credentials are configured")
			})
		}
	})

	t.Run("credentials with the proxy return placeholders", func(t *testing.T) {
		// Arrange
		in := Input{ProxyMode: docker.ProxyMonitor, Credentials: resolved}
		mounts := mountSet{toolHome: t.TempDir(), containerHome: "/home/agent"}

		// Act
		env, err := credentialSetup(in, mounts, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"EXAMPLE_API_KEY=" + credentials.Placeholder}, env)
	})
}

func Test_credentialEnv(t *testing.T) {
	t.Run("sets each env var to the placeholder", func(t *testing.T) {
		// Arrange
		resolved := []credentials.Resolved{{Env: []string{"EXAMPLE_TOKEN", "EXAMPLE_KEY"}}, {Env: []string{"OTHER_TOKEN"}}}

		// Act
		env, err := credentialEnv(resolved, nil, false)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{
			"EXAMPLE_TOKEN=" + credentials.Placeholder,
			"EXAMPLE_KEY=" + credentials.Placeholder,
			"OTHER_TOKEN=" + credentials.Placeholder,
		}, env)
	})

	t.Run("refuses an env entry for the same name", func(t *testing.T) {
		for _, entry := range []string{"EXAMPLE_TOKEN=value", "EXAMPLE_TOKEN"} {
			t.Run(entry, func(t *testing.T) {
				// Arrange
				resolved := []credentials.Resolved{{Env: []string{"EXAMPLE_TOKEN"}}}

				// Act
				_, err := credentialEnv(resolved, []string{entry}, false)

				// Assert
				assert.ErrorContains(t, err, "set by a proxy credential")
			})
		}
	})

	t.Run("refuses names agentic manages", func(t *testing.T) {
		for _, name := range []string{"HTTPS_PROXY", "TOOL_HOME", "AGENTIC_PROXY_CA", "GIT_SSL_CAINFO"} {
			t.Run(name, func(t *testing.T) {
				// Arrange
				resolved := []credentials.Resolved{{Env: []string{name}}}

				// Act
				_, err := credentialEnv(resolved, nil, false)

				// Assert
				assert.ErrorContains(t, err, "managed by agentic")
			})
		}
	})

	t.Run("refuses docker names only with dind", func(t *testing.T) {
		// Arrange
		resolved := []credentials.Resolved{{Env: []string{"DOCKER_HOST"}}}

		// Act
		_, withoutDind := credentialEnv(resolved, nil, false)
		_, withDind := credentialEnv(resolved, nil, true)

		// Assert
		assert.NoError(t, withoutDind)
		assert.ErrorContains(t, withDind, "managed by agentic")
	})
}

func Test_checkCredentialPaths(t *testing.T) {
	t.Run("secret outside every mount is accepted", func(t *testing.T) {
		// Arrange
		resolved := []credentials.Resolved{{Path: filepath.Join(t.TempDir(), "token")}}
		mounts := mountSet{
			volumes:       []string{t.TempDir() + ":/data", "agentic-cache:/cache"},
			toolHome:      t.TempDir(),
			containerHome: "/home/agent",
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
		mounts := mountSet{volumes: []string{dir + ":/data:ro"}, toolHome: t.TempDir(), containerHome: "/home/agent"}

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
			volumes:       []string{"$TOOL_HOME/claude:$CONTAINER_HOME/.claude"},
			toolHome:      toolHome,
			containerHome: "/home/agent",
		}

		// Act
		err := checkCredentialPaths(resolved, mounts)

		// Assert
		assert.ErrorContains(t, err, "which the tool container can access")
	})

	t.Run("secret that is itself a mounted secret is refused", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "token")
		resolved := []credentials.Resolved{{Path: path}}
		mounts := mountSet{secrets: []string{"token:" + path}, toolHome: t.TempDir(), containerHome: "/home/agent"}

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
		mounts := mountSet{volumes: []string{mounted + ":/data"}, toolHome: t.TempDir(), containerHome: "/home/agent"}

		// Act
		err := checkCredentialPaths(resolved, mounts)

		// Assert
		assert.ErrorContains(t, err, "which the tool container can access")
	})
}

func Test_checkProxyTrust(t *testing.T) {
	target := Target{ToolName: "claude", ImageName: "agentic-claude"}

	t.Run("an image with proxy trust passes", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: func(string) (*docker.ImageInfo, error) { return &docker.ImageInfo{ProxyTrust: true}, nil },
		}

		// Act
		err := New(d).checkProxyTrust(target)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("an image without proxy trust is refused with the rebuild command", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: func(string) (*docker.ImageInfo, error) { return &docker.ImageInfo{}, nil },
		}

		// Act
		err := New(d).checkProxyTrust(target)

		// Assert
		assert.ErrorContains(t, err, `agentic update claude`)
	})

	t.Run("a missing image passes, as it is built fresh", func(t *testing.T) {
		// Act
		err := New(&fakeDocker{}).checkProxyTrust(target)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("an inspect error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: func(string) (*docker.ImageInfo, error) { return nil, fmt.Errorf("stub: daemon down") },
		}

		// Act
		err := New(d).checkProxyTrust(target)

		// Assert
		assert.ErrorContains(t, err, "daemon down")
	})

	t.Run("a skipped entrypoint warns without inspecting the image", func(t *testing.T) {
		// Arrange
		stderr := stubErrLog(t)
		inspected := false
		d := &fakeDocker{
			inspectImage: func(string) (*docker.ImageInfo, error) {
				inspected = true
				return &docker.ImageInfo{}, nil
			},
		}
		skipped := Target{ToolName: "claude", ImageName: "agentic-claude", SkipEntrypoint: true}

		// Act
		err := New(d).checkProxyTrust(skipped)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, stderr.String(), "proxy CA is not trusted")
		assert.False(t, inspected)
	})
}

func Test_describeInjection(t *testing.T) {
	// Arrange
	creds := []credentials.Resolved{
		{
			Proxy: []proxy.Credential{
				{Hosts: []string{"api.example.test"}, Rules: []proxy.InjectRule{{Header: "X-Api-Key", Value: "test-secret"}}},
				{Hosts: []string{"git.example.test"}, Rules: []proxy.InjectRule{{Header: "Authorization", Value: "Basic test-secret"}}},
			},
			Source: "/home/user/.secrets/example",
		},
		{
			Proxy:  []proxy.Credential{{Hosts: []string{"other.example.test"}, Rules: []proxy.InjectRule{{Header: "Authorization", Value: "Bearer test-secret"}}}},
			Source: "/home/user/.secrets/other",
		},
	}

	// Act
	desc := describeInjection(creds)

	// Assert
	assert.Equal(t, "api.example.test, git.example.test (/home/user/.secrets/example); other.example.test (/home/user/.secrets/other)", desc)
	assert.NotContains(t, desc, "test-secret")
}
