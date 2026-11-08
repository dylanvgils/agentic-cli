package docker

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_StartPublishedProxy(t *testing.T) {
	client := newTestClient()
	stubProxySettleTime(t, client, 0)
	newSpec := func(t *testing.T) sandbox.RunSpec {
		return sandbox.RunSpec{
			User:     &testUser,
			ToolHome: t.TempDir(),
			Egress:   sandbox.EgressPolicy{Mode: sandbox.EgressEnforce},
		}
	}

	t.Run("publishes the proxy on loopback with a token only its env file holds", func(t *testing.T) {
		// Arrange
		calls, tokenFile := stubPublishedProxyDocker(t, client, "unix:///var/run/docker.sock")
		spec := newSpec(t)

		// Act
		published, err := client.StartPublishedProxy(spec, filepath.Join(t.TempDir(), "proxy_abc.jsonl"), sandbox.RuntimeMicrosandbox)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "49153", published.Port)
		assert.Regexp(t, `^[0-9a-f]{64}$`, published.Token)
		create := findCall(calls(), "create")
		assert.Contains(t, create, "--network="+NetworkName)
		assert.Contains(t, create, "--publish=127.0.0.1::3128")
		assert.Contains(t, create, "--label="+LabelOwner+"=microsandbox:pid:"+strconv.Itoa(os.Getpid()))
		assert.False(t, hasArgWithPrefix(create, "--network-alias"), "no run network to alias on")
		assert.False(t, hasArgWithPrefix(create, "--env="+proxy.EnvClientNets), "the token admits clients instead")
		assert.NotContains(t, strings.Join(create, " "), published.Token)
		path, content := tokenFile()
		assert.Equal(t, proxy.EnvAuthToken+"="+published.Token+"\n", content)
		assert.NoFileExists(t, path, "removed once the container is created")
		assert.Nil(t, findCall(calls(), "network", "connect"), "already on the egress network")
	})

	t.Run("returns the CA and placeholders with credentials", func(t *testing.T) {
		// Arrange
		stubPublishedProxyDocker(t, client, "unix:///var/run/docker.sock")
		stubDockerRunWithStdinCapture(t, client, nil)
		spec := newSpec(t)
		spec.Credentials = testCredentials

		// Act
		published, err := client.StartPublishedProxy(spec, filepath.Join(t.TempDir(), "proxy_abc.jsonl"), sandbox.RuntimeMicrosandbox)

		// Assert
		require.NoError(t, err)
		assert.NotEmpty(t, published.CAPEM)
		require.Len(t, published.Env, 1)
		assert.Regexp(t, `^EXAMPLE_API_KEY=agentic-[0-9a-f]{24}$`, published.Env[0])
	})

	t.Run("stop removes the proxy", func(t *testing.T) {
		// Arrange
		calls, _ := stubPublishedProxyDocker(t, client, "unix:///var/run/docker.sock")
		published, err := client.StartPublishedProxy(newSpec(t), filepath.Join(t.TempDir(), "proxy_abc.jsonl"), sandbox.RuntimeMicrosandbox)
		require.NoError(t, err)

		// Act
		published.Stop()

		// Assert
		assert.NotNil(t, findCall(calls(), "rm", "--force", "--volumes"))
	})

	t.Run("removes the proxy when its port can't be read", func(t *testing.T) {
		// Arrange
		calls, _ := stubPublishedProxyDocker(t, client, "unix:///var/run/docker.sock", "port")

		// Act
		_, err := client.StartPublishedProxy(newSpec(t), filepath.Join(t.TempDir(), "proxy_abc.jsonl"), sandbox.RuntimeMicrosandbox)

		// Assert
		require.ErrorContains(t, err, "read the proxy's published port")
		assert.NotNil(t, findCall(calls(), "rm", "--force", "--volumes"))
	})

	t.Run("refuses a remote Docker context before creating anything", func(t *testing.T) {
		// Arrange
		calls, _ := stubPublishedProxyDocker(t, client, "tcp://docker.example.test:2376")

		// Act
		_, err := client.StartPublishedProxy(newSpec(t), filepath.Join(t.TempDir(), "proxy_abc.jsonl"), sandbox.RuntimeMicrosandbox)

		// Assert
		require.ErrorContains(t, err, "isn't on this machine")
		assert.Nil(t, findCall(calls(), "create"))
	})

	t.Run("dry run prints placeholders and creates nothing", func(t *testing.T) {
		// Arrange
		calls, _ := stubPublishedProxyDocker(t, client, "unix:///var/run/docker.sock")
		spec := newSpec(t)
		spec.DryRun = true
		spec.Credentials = testCredentials

		// Act
		var published PublishedProxy
		out := iostreamstest.CaptureOut(t, func() {
			var err error
			published, err = client.StartPublishedProxy(spec, filepath.Join(t.TempDir(), "proxy_abc.jsonl"), sandbox.RuntimeMicrosandbox)
			require.NoError(t, err)
		})

		// Assert
		assert.Equal(t, PublishedProxy{
			Port:  dryRunPortPlaceholder,
			Token: dryRunTokenPlaceholder,
			CAPEM: []byte(dryRunCAPlaceholder),
			Env:   published.Env,
		}, published)
		assert.Contains(t, out, "--env-file="+dryRunTokenFilePlaceholder)
		assert.Contains(t, out, "--publish=127.0.0.1::3128")
		assert.NotContains(t, out, "test-secret")
		assert.Empty(t, calls())
	})
}

func Test_parsePublishedPort(t *testing.T) {
	t.Run("reads the first line's port", func(t *testing.T) {
		// Act
		port, err := parsePublishedPort("127.0.0.1:49153\n[::1]:49154\n")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "49153", port)
	})

	t.Run("refuses output without a valid port", func(t *testing.T) {
		for _, out := range []string{"", "127.0.0.1:", "127.0.0.1:http", "127.0.0.1:" + strconv.Itoa(70000)} {
			// Act
			_, err := parsePublishedPort(out)

			// Assert
			assert.ErrorContains(t, err, "unexpected docker port output", out)
		}
	})
}

func TestPublishedProxy_Stop(t *testing.T) {
	// Act + Assert
	assert.NotPanics(t, PublishedProxy{}.Stop)
}
