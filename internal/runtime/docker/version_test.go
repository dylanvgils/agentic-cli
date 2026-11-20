package docker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractVersion(t *testing.T) {
	for _, tc := range []struct {
		name   string
		output string
		want   string
	}{
		{name: "semver", output: "v24.0.0\n", want: "24.0.0"},
		{name: "prefixed output", output: "go version go1.21.0 linux/amd64\n", want: "1.21.0"},
		{name: "docker cli output", output: "Docker version 29.8.2, build 1a2b3c4\n", want: "29.8.2"},
		{name: "windows line ending", output: "1.0.0\r\n", want: "1.0.0"},
		{name: "multi line uses first line", output: "1.2.3\n4.5.6\n", want: "1.2.3"},
		{name: "no version returns empty", output: "no version here\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := extractVersion(tc.output)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestClient_Version(t *testing.T) {
	client := newTestClient()

	t.Run("returns the trimmed client version", func(t *testing.T) {
		// Arrange
		var capturedArgs []string
		stubDockerRun(t, client, func(args ...string) (string, error) {
			capturedArgs = args
			return "29.9.0\n", nil
		})

		// Act
		result, err := client.Version()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "29.9.0", result)
		assert.Equal(t, []string{"version", "--format={{.Client.Version}}"}, capturedArgs)
	})

	t.Run("docker error propagates", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", fmt.Errorf("docker not found"))

		// Act
		_, err := client.Version()

		// Assert
		require.Error(t, err)
	})
}

func TestClient_ServerVersion(t *testing.T) {
	client := newTestClient()

	t.Run("returns the engine component", func(t *testing.T) {
		// Arrange
		var capturedArgs []string
		stubDockerRun(t, client, func(args ...string) (string, error) {
			capturedArgs = args
			return `[{"Name":"containerd","Version":"v2.4.1"},{"Name":"Engine","Version":"29.9.0","Details":{}}]` + "\n", nil
		})

		// Act
		engine, version, err := client.ServerVersion()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "Engine", engine)
		assert.Equal(t, "29.9.0", version)
		assert.Equal(t, []string{"version", "--format={{json .Server.Components}}"}, capturedArgs)
	})

	t.Run("unparseable output is an error", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "Engine 29.9.0\n", nil)

		// Act
		_, _, err := client.ServerVersion()

		// Assert
		assert.ErrorContains(t, err, "parse docker version output")
	})

	t.Run("docker error propagates", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", fmt.Errorf("cannot connect to the docker daemon"))

		// Act
		_, _, err := client.ServerVersion()

		// Assert
		assert.EqualError(t, err, "cannot connect to the docker daemon")
	})
}

func Test_engineComponent(t *testing.T) {
	for _, tc := range []struct {
		name       string
		components []serverComponent
		want       serverComponent
	}{
		{
			name: "picks a component named like an engine",
			components: []serverComponent{
				{"Conmon", "2.1.12"},
				{"Podman Engine", "5.2.0"},
			},
			want: serverComponent{"Podman Engine", "5.2.0"},
		},
		{
			name: "falls back to the first component",
			components: []serverComponent{
				{"Daemon", "1.0.0"},
				{"runtime", "2.0.0"},
			},
			want: serverComponent{"Daemon", "1.0.0"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got, err := engineComponent(tc.components)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	t.Run("no components is an error", func(t *testing.T) {
		// Act
		_, err := engineComponent(nil)

		// Assert
		assert.EqualError(t, err, "docker version lists no server components")
	})
}

func TestRunVersionScript(t *testing.T) {
	client := newTestClient()

	t.Run("returns the version detected without network, so the tool can't self-update mid-detection", func(t *testing.T) {
		// Arrange
		var capturedArgs []string
		stubDockerRun(t, client, func(args ...string) (string, error) {
			capturedArgs = args
			return "1.2.3\n", nil
		})

		// Act
		result := client.runVersionScript("agentic-claude", "agentic-version-claude")

		// Assert
		assert.Equal(t, "1.2.3", result)
		assert.Contains(t, capturedArgs, "--network=none")
	})

	t.Run("docker run error returns empty", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", fmt.Errorf("not found"))

		// Act
		result := client.runVersionScript("agentic-claude", "agentic-version-claude")

		// Assert
		assert.Equal(t, "", result)
	})

}

func TestCollectExtraVersions(t *testing.T) {
	// Arrange
	client := newTestClient()
	stubDockerRunFixed(t, client, "21.0.1\n", nil)

	// Act
	result := client.collectExtraVersions("agentic-base", []string{"java", "python"})

	// Assert
	assert.Equal(t, map[string]string{"java": "21.0.1", "python": "21.0.1"}, result)
}
