package run

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Service_Run(t *testing.T) {
	t.Run("docker runs the container", func(t *testing.T) {
		// Arrange
		var ranSpec sandbox.RunSpec
		microsandboxRan := false
		svc := New(&fakeDocker{
			startSidecars: func(rs sandbox.RunSpec) (docker.Sidecars, error) {
				ranSpec = rs
				return docker.Sidecars{}, nil
			},
		}, &fakeMicrosandbox{run: func(sandbox.RunSpec) error {
			microsandboxRan = true
			return nil
		}})

		// Act
		err := svc.Run(runtimeRequest(sandbox.RuntimeDocker), sandbox.RunSpec{Image: "agentic-claude"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude", ranSpec.Image)
		assert.False(t, microsandboxRan)
	})

	t.Run("microsandbox runs through the microsandbox client", func(t *testing.T) {
		// Arrange
		dockerRan := false
		var ranSpec sandbox.RunSpec
		d := &fakeDocker{
			inspectImage: inspectImageID("abc123"),
			runTool: func(docker.Sidecars) error {
				dockerRan = true
				return nil
			},
		}
		svc := New(d, &fakeMicrosandbox{run: func(rs sandbox.RunSpec) error {
			ranSpec = rs
			return nil
		}})

		// Act
		err := svc.Run(runtimeRequest(sandbox.RuntimeMicrosandbox), sandbox.RunSpec{Image: "agentic-claude", DryRun: true})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude:abc123", ranSpec.Image)
		assert.False(t, dockerRan)
	})

	t.Run("a spec relying on a feature the runtime lacks never reaches it", func(t *testing.T) {
		// Arrange
		ran := false
		svc := New(&fakeDocker{inspectImage: inspectImageID("abc123")}, &fakeMicrosandbox{run: func(sandbox.RunSpec) error {
			ran = true
			return nil
		}})
		rs := sandbox.RunSpec{Image: "agentic-claude", Secrets: []string{"token:/run/secrets/token"}}

		// Act
		err := svc.Run(runtimeRequest(sandbox.RuntimeMicrosandbox), rs)

		// Assert
		assert.ErrorContains(t, err, "can't enforce secret mounts")
		assert.False(t, ran)
	})

	t.Run("mounts that changed since the spec was built never reach the runtime", func(t *testing.T) {
		// Arrange
		ws := chdirSymlinkWorkspace(t)
		ran := false
		svc := New(&fakeDocker{inspectImage: inspectImageID("abc123")}, &fakeMicrosandbox{run: func(sandbox.RunSpec) error {
			ran = true
			return nil
		}})
		rs := sandbox.RunSpec{Image: "agentic-claude", ToolHome: t.TempDir(), Volumes: []string{"$PWD/link:/data:ro"}}

		// Act
		err := svc.Run(runtimeRequest(sandbox.RuntimeMicrosandbox), rs)

		// Assert
		assert.ErrorContains(t, err, "goes through workspace symlink "+filepath.Join(ws.dir, "link"))
		assert.False(t, ran)
	})

	t.Run("unset runtime runs on docker", func(t *testing.T) {
		// Arrange
		dockerRan := false
		svc := New(&fakeDocker{runTool: func(docker.Sidecars) error {
			dockerRan = true
			return nil
		}}, nil)

		// Act
		err := svc.Run(runtimeRequest(""), sandbox.RunSpec{Image: "agentic-claude"})

		// Assert
		require.NoError(t, err)
		assert.True(t, dockerRan)
	})

	t.Run("unknown runtime returns error", func(t *testing.T) {
		// Arrange
		svc := New(&fakeDocker{}, nil)

		// Act
		err := svc.Run(runtimeRequest(sandbox.Runtime("podman")), sandbox.RunSpec{})

		// Assert
		assert.ErrorContains(t, err, `unknown runtime "podman"`)
	})
}

func Test_Service_runner(t *testing.T) {
	svc := New(&fakeDocker{}, &fakeMicrosandbox{})

	t.Run("docker and the empty runtime get the docker runner", func(t *testing.T) {
		// Act
		docker, dockerErr := svc.runner(sandbox.RuntimeDocker)
		empty, emptyErr := svc.runner("")

		// Assert
		require.NoError(t, dockerErr)
		require.NoError(t, emptyErr)
		assert.Equal(t, dockerRunner{docker: svc.docker}, docker)
		assert.Equal(t, dockerRunner{docker: svc.docker}, empty)
	})

	t.Run("microsandbox gets the microsandbox runner", func(t *testing.T) {
		// Act
		r, err := svc.runner(sandbox.RuntimeMicrosandbox)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, microsandboxRunner{docker: svc.docker, microsandbox: svc.microsandbox}, r)
	})

	t.Run("every parseable runtime has a runner", func(t *testing.T) {
		for _, runtime := range sandbox.Runtimes {
			// Act
			_, err := svc.runner(runtime)

			// Assert
			assert.NoError(t, err, runtime)
		}
	})

	t.Run("an unknown runtime is an error", func(t *testing.T) {
		// Act
		_, err := svc.runner("podman")

		// Assert
		assert.ErrorContains(t, err, `unknown runtime "podman"`)
	})
}

func Test_Service_capabilities(t *testing.T) {
	dockerCaps := sandbox.Capabilities{Proxy: true}
	msbCaps := sandbox.Capabilities{CredentialInjection: true}
	svc := New(
		&fakeDocker{capabilities: func() sandbox.Capabilities { return dockerCaps }},
		&fakeMicrosandbox{capabilities: func() sandbox.Capabilities { return msbCaps }},
	)

	t.Run("docker and the empty runtime ask docker", func(t *testing.T) {
		// Act
		docker := svc.capabilities(sandbox.RuntimeDocker)
		empty := svc.capabilities("")

		// Assert
		assert.Equal(t, dockerCaps, docker)
		assert.Equal(t, dockerCaps, empty)
	})

	t.Run("microsandbox asks msb", func(t *testing.T) {
		// Act
		caps := svc.capabilities(sandbox.RuntimeMicrosandbox)

		// Assert
		assert.Equal(t, msbCaps, caps)
	})

	t.Run("an unknown runtime offers nothing", func(t *testing.T) {
		// Act
		caps := svc.capabilities("podman")

		// Assert
		assert.Equal(t, sandbox.Capabilities{}, caps)
	})
}

func Test_Service_checkRuntime(t *testing.T) {
	t.Run("docker passes every feature", func(t *testing.T) {
		// Arrange
		req := newCredentialRequest(t)
		req.Flags.Proxy.ProxyFlag = true
		req.Flags.Dind.DindFlag = true

		// Act
		err := New(&fakeDocker{}, nil).checkRuntime(req)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("a missing capability stops before the msb checks", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Flags.Runtime = sandbox.RuntimeMicrosandbox
		req.Flags.Proxy.MonitorFlag = true
		checked := false
		m := &fakeMicrosandbox{available: func() error {
			checked = true
			return nil
		}}

		// Act
		err := New(&fakeDocker{}, m).checkRuntime(req)

		// Assert
		assert.ErrorContains(t, err, "does not support proxy monitor mode (--proxy-monitor)")
		assert.False(t, checked)
	})

	t.Run("an unknown runtime is an error", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Flags.Runtime = "podman"

		// Act
		err := New(&fakeDocker{}, nil).checkRuntime(req)

		// Assert
		assert.ErrorContains(t, err, `unknown runtime "podman"`)
	})

	t.Run("microsandbox runs its own checks", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Flags.Runtime = sandbox.RuntimeMicrosandbox
		m := &fakeMicrosandbox{available: func() error { return errors.New("msb not found on PATH") }}

		// Act
		err := New(&fakeDocker{}, m).checkRuntime(req)

		// Assert
		assert.EqualError(t, err, "msb not found on PATH")
	})
}
