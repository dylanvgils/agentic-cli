package run

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Service_Run(t *testing.T) {
	t.Run("docker runs the container", func(t *testing.T) {
		// Arrange
		var ranSpec docker.RunSpec
		var ranArgs []string
		microsandboxRan := false
		svc := New(&fakeDocker{runContainer: func(rs docker.RunSpec, toolArgs []string) error {
			ranSpec, ranArgs = rs, toolArgs
			return nil
		}}, &fakeMicrosandbox{run: func(docker.RunSpec, []string) error {
			microsandboxRan = true
			return nil
		}})

		// Act
		err := svc.Run(runtimeRequest(RuntimeDocker), docker.RunSpec{Image: "agentic-claude"}, []string{"--help"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude", ranSpec.Image)
		assert.Equal(t, []string{"--help"}, ranArgs)
		assert.False(t, microsandboxRan)
	})

	t.Run("microsandbox runs through the microsandbox client", func(t *testing.T) {
		// Arrange
		dockerRan := false
		var ranSpec docker.RunSpec
		d := &fakeDocker{
			inspectImage: inspectImageID("abc123"),
			runContainer: func(docker.RunSpec, []string) error {
				dockerRan = true
				return nil
			},
		}
		svc := New(d, &fakeMicrosandbox{run: func(rs docker.RunSpec, _ []string) error {
			ranSpec = rs
			return nil
		}})

		// Act
		err := svc.Run(runtimeRequest(RuntimeMicrosandbox), docker.RunSpec{Image: "agentic-claude", DryRun: true}, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude:abc123", ranSpec.Image)
		assert.False(t, dockerRan)
	})

	t.Run("empty runtime runs on docker", func(t *testing.T) {
		// Arrange
		dockerRan := false
		svc := New(&fakeDocker{runContainer: func(docker.RunSpec, []string) error {
			dockerRan = true
			return nil
		}}, nil)

		// Act
		err := svc.Run(Request{}, docker.RunSpec{Image: "agentic-claude"}, nil)

		// Assert
		require.NoError(t, err)
		assert.True(t, dockerRan)
	})

	t.Run("unknown runtime returns error", func(t *testing.T) {
		// Arrange
		svc := New(&fakeDocker{}, nil)

		// Act
		err := svc.Run(runtimeRequest(Runtime("podman")), docker.RunSpec{}, nil)

		// Assert
		assert.ErrorContains(t, err, `unknown runtime "podman"`)
	})
}

func TestParseRuntime(t *testing.T) {
	t.Run("known name returns its runtime", func(t *testing.T) {
		// Act
		runtime, err := ParseRuntime("microsandbox")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, RuntimeMicrosandbox, runtime)
	})

	t.Run("unknown name lists the supported runtimes", func(t *testing.T) {
		// Act
		_, err := ParseRuntime("podman")

		// Assert
		assert.EqualError(t, err, `unknown runtime "podman": expected docker or microsandbox`)
	})
}
