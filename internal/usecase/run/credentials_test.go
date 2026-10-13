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
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
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
	resolved := []credentials.Resolved{{Env: []string{"EXAMPLE_API_KEY"}, Placeholder: "agentic-0123456789abcdef"}}
	proxyInjects := sandbox.Capabilities{Proxy: true, CredentialInjection: true, CredentialsViaProxy: true}
	runtimeInjects := sandbox.Capabilities{CredentialInjection: true}

	t.Run("no credentials is a no-op even with the proxy off", func(t *testing.T) {
		// Act
		env, err := credentialSetup(Input{}, proxyInjects, mountSet{}, nil)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, env)
	})

	t.Run("credentials without the proxy are an error", func(t *testing.T) {
		// Arrange
		in := Input{Credentials: resolved}

		// Act
		_, err := credentialSetup(in, proxyInjects, mountSet{}, nil)

		// Assert
		assert.ErrorContains(t, err, "need the egress proxy")
	})

	t.Run("credentials with the proxy return placeholders", func(t *testing.T) {
		// Arrange
		in := Input{ProxyMode: sandbox.ProxyMonitor, Credentials: resolved}
		mounts := mountSet{toolHome: t.TempDir(), containerHome: "/home/agent"}

		// Act
		env, err := credentialSetup(in, proxyInjects, mounts, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"EXAMPLE_API_KEY=agentic-0123456789abcdef"}, env)
	})

	t.Run("a credential covering git adds the git helper for every runtime", func(t *testing.T) {
		// Arrange
		git := []credentials.Resolved{{Env: []string{"GITHUB_TOKEN"}, Placeholder: "agentic-0123456789abcdef", Git: &credentials.GitAuth{Host: "github.com", User: "x-access-token"}}}
		mounts := mountSet{toolHome: t.TempDir(), containerHome: "/home/agent"}

		// Act
		proxyEnv, err := credentialSetup(Input{ProxyMode: sandbox.ProxyEnforce, Credentials: git}, proxyInjects, mounts, nil)
		require.NoError(t, err)
		runtimeEnv, err := credentialSetup(Input{Credentials: git}, runtimeInjects, mounts, nil)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, proxyEnv, "GIT_CONFIG_COUNT=1")
		assert.Contains(t, runtimeEnv, "GIT_CONFIG_COUNT=1")
		assert.NotContains(t, runtimeEnv, "GITHUB_TOKEN=agentic-0123456789abcdef")
	})

	t.Run("a runtime that injects itself needs no proxy and gets no placeholders, but the same checks", func(t *testing.T) {
		// Arrange
		in := Input{Credentials: resolved}
		mounts := mountSet{toolHome: t.TempDir(), containerHome: "/home/agent"}

		// Act
		env, err := credentialSetup(in, runtimeInjects, mounts, nil)
		_, overrideErr := credentialSetup(in, runtimeInjects, mounts, []string{"EXAMPLE_API_KEY=value"})

		// Assert
		require.NoError(t, err)
		assert.Empty(t, env)
		assert.ErrorContains(t, overrideErr, "set by a proxy credential")
	})
}

func Test_sandboxCredentials(t *testing.T) {
	// Arrange
	resolved := []credentials.Resolved{{
		Hosts:       []string{"api.example.test", "git.example.test"},
		Headers:     []string{"Authorization"},
		Env:         []string{"EXAMPLE_TOKEN", "EXAMPLE_KEY"},
		Placeholder: "agentic-0123456789abcdef",
		Secret:      "test-secret",
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

func Test_proxyCredentials(t *testing.T) {
	// Arrange
	resolved := []credentials.Resolved{{
		Hosts:       []string{"api.example.test"},
		Headers:     []string{"X-Api-Key"},
		Env:         []string{"EXAMPLE_TOKEN"},
		Placeholder: "agentic-0123456789abcdef",
		Secret:      "test-secret",
	}}

	// Act
	creds := proxyCredentials(resolved)

	// Assert
	assert.Equal(t, []proxy.Credential{{Hosts: []string{"api.example.test"}, Headers: []string{"X-Api-Key"}, Placeholder: "agentic-0123456789abcdef", Secret: "test-secret"}}, creds)
}

func Test_credentialEnv(t *testing.T) {
	// Arrange
	resolved := []credentials.Resolved{
		{Env: []string{"EXAMPLE_TOKEN", "EXAMPLE_KEY"}, Placeholder: "agentic-aaaaaaaaaaaaaaaa"},
		{Env: []string{"OTHER_TOKEN"}, Placeholder: "agentic-bbbbbbbbbbbbbbbb"},
	}

	// Act
	env := credentialEnv(resolved)

	// Assert
	assert.Equal(t, []string{
		"EXAMPLE_TOKEN=agentic-aaaaaaaaaaaaaaaa",
		"EXAMPLE_KEY=agentic-aaaaaaaaaaaaaaaa",
		"OTHER_TOKEN=agentic-bbbbbbbbbbbbbbbb",
	}, env)
}

func Test_gitHelperEnv(t *testing.T) {
	t.Run("sets a helper per credential covering git", func(t *testing.T) {
		// Arrange
		resolved := []credentials.Resolved{
			{Env: []string{"GITHUB_TOKEN", "GH_TOKEN"}, Git: &credentials.GitAuth{Host: "github.com", User: "x-access-token"}},
			{Env: []string{"OTHER_TOKEN"}},
			{Env: []string{"GITEA_TOKEN"}, Git: &credentials.GitAuth{Host: "git.example.test", User: "oauth2"}},
		}

		// Act
		env := gitHelperEnv(resolved)

		// Assert
		assert.Equal(t, []string{
			"GIT_CONFIG_COUNT=2",
			"GIT_CONFIG_KEY_0=credential.https://github.com.helper",
			`GIT_CONFIG_VALUE_0=!f() { echo username=x-access-token; echo "password=$GITHUB_TOKEN"; }; f`,
			"GIT_CONFIG_KEY_1=credential.https://git.example.test.helper",
			`GIT_CONFIG_VALUE_1=!f() { echo username=oauth2; echo "password=$GITEA_TOKEN"; }; f`,
		}, env)
	})

	t.Run("nil without a credential covering git", func(t *testing.T) {
		// Act
		env := gitHelperEnv([]credentials.Resolved{{Env: []string{"OTHER_TOKEN"}}})

		// Assert
		assert.Nil(t, env)
	})
}

func Test_checkCredentialConfig(t *testing.T) {
	t.Run("accepts presets and entries with env", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		rc.Run.Proxy.Credentials = []config.RCCredential{{Preset: "github"}, {Hosts: []string{"api.example.test"}, Env: []string{"EXAMPLE_TOKEN"}}}

		// Act
		err := checkCredentialConfig(rc)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("refuses an entry without env", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		rc.Run.Proxy.Credentials = []config.RCCredential{{Hosts: []string{"api.example.test"}}}

		// Act
		err := checkCredentialConfig(rc)

		// Assert
		assert.EqualError(t, err, "credential for api.example.test needs env, as the tool must send the placeholder")
	})

	t.Run("warns that format is ignored", func(t *testing.T) {
		// Arrange
		errLog := stubErrLog(t)
		rc := &config.AgenticRC{}
		rc.Run.Proxy.Credentials = []config.RCCredential{{Hosts: []string{"api.example.test"}, Env: []string{"EXAMPLE_TOKEN"}, Format: "Token %s"}}

		// Act
		err := checkCredentialConfig(rc)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, errLog.String(), "the credential format for api.example.test is ignored")
	})
}

func Test_checkCredentialEnv(t *testing.T) {
	t.Run("accepts names nothing else sets", func(t *testing.T) {
		// Arrange
		resolved := []credentials.Resolved{{Env: []string{"EXAMPLE_TOKEN"}}}

		// Act
		err := checkCredentialEnv(resolved, []string{"OTHER=value"}, false)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("refuses an env entry for a proxy trust var", func(t *testing.T) {
		for _, entry := range []string{"AGENTIC_PROXY_CA=test-ca", "SSL_CERT_FILE=/certs/example.pem", "NODE_EXTRA_CA_CERTS"} {
			t.Run(entry, func(t *testing.T) {
				// Arrange
				resolved := []credentials.Resolved{{Env: []string{"EXAMPLE_TOKEN"}}}

				// Act
				err := checkCredentialEnv(resolved, []string{entry}, false)

				// Assert
				assert.ErrorContains(t, err, "set by agentic when proxy credentials are configured")
			})
		}
	})

	t.Run("refuses an env entry for the same name", func(t *testing.T) {
		for _, entry := range []string{"EXAMPLE_TOKEN=value", "EXAMPLE_TOKEN"} {
			t.Run(entry, func(t *testing.T) {
				// Arrange
				resolved := []credentials.Resolved{{Env: []string{"EXAMPLE_TOKEN"}}}

				// Act
				err := checkCredentialEnv(resolved, []string{entry}, false)

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
				err := checkCredentialEnv(resolved, nil, false)

				// Assert
				assert.ErrorContains(t, err, "managed by agentic")
			})
		}
	})

	t.Run("refuses docker names only with dind", func(t *testing.T) {
		// Arrange
		resolved := []credentials.Resolved{{Env: []string{"DOCKER_HOST"}}}

		// Act
		withoutDind := checkCredentialEnv(resolved, nil, false)
		withDind := checkCredentialEnv(resolved, nil, true)

		// Assert
		assert.NoError(t, withoutDind)
		assert.ErrorContains(t, withDind, "managed by agentic")
	})

	t.Run("refuses git config env when a credential covers git", func(t *testing.T) {
		// Arrange
		resolved := []credentials.Resolved{{Env: []string{"GITHUB_TOKEN"}, Git: &credentials.GitAuth{Host: "github.com", User: "x-access-token"}}}

		// Act
		err := checkCredentialEnv(resolved, []string{"GIT_CONFIG_COUNT=0"}, false)

		// Assert
		assert.ErrorContains(t, err, `"GIT_CONFIG_COUNT" is set by agentic for the git credential helper`)
	})

	t.Run("allows git config env without a credential covering git", func(t *testing.T) {
		// Arrange
		resolved := []credentials.Resolved{{Env: []string{"EXAMPLE_TOKEN"}}}

		// Act
		err := checkCredentialEnv(resolved, []string{"GIT_CONFIG_COUNT=0"}, false)

		// Assert
		assert.NoError(t, err)
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
		err := New(d, nil).checkProxyTrust(target)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("an image without proxy trust is refused with the rebuild command", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: func(string) (*docker.ImageInfo, error) { return &docker.ImageInfo{}, nil },
		}

		// Act
		err := New(d, nil).checkProxyTrust(target)

		// Assert
		assert.ErrorContains(t, err, `agentic update claude`)
	})

	t.Run("a missing image passes, as it is built fresh", func(t *testing.T) {
		// Act
		err := New(&fakeDocker{}, nil).checkProxyTrust(target)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("an inspect error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: func(string) (*docker.ImageInfo, error) { return nil, fmt.Errorf("stub: daemon down") },
		}

		// Act
		err := New(d, nil).checkProxyTrust(target)

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
		err := New(d, nil).checkProxyTrust(skipped)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, stderr.String(), "proxy CA is not trusted")
		assert.False(t, inspected)
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
