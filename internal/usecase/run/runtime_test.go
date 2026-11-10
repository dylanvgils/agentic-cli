package run

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Service_Run(t *testing.T) {
	t.Run("docker runs the container", func(t *testing.T) {
		// Arrange
		var ranSpec sandbox.RunSpec
		d := &fakeDocker{startSidecars: func(rs sandbox.RunSpec, _ string) (docker.Sidecars, error) {
			ranSpec = rs
			return docker.Sidecars{}, nil
		}}
		m := &fakeMicrosandbox{}
		msbRan := stubRunImage(m)

		// Act
		err := newTestService(d, m).Run(runtimeRequest(sandbox.RuntimeDocker), sandbox.RunSpec{Image: "agentic-claude"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude", ranSpec.Image)
		assert.Empty(t, *msbRan)
		_, idErr := sandbox.ParseRunID(ranSpec.RunID.String())
		assert.NoError(t, idErr, "every run gets a fresh run id")
	})

	t.Run("microsandbox runs through the microsandbox client", func(t *testing.T) {
		// Arrange
		dockerRan := false
		d := &fakeDocker{
			inspectImage: inspectImageID("abc123"),
			runTool: func(docker.Sidecars) error {
				dockerRan = true
				return nil
			},
		}
		m := &fakeMicrosandbox{}
		ran := stubRunImage(m)

		// Act
		err := newTestService(d, m).Run(runtimeRequest(sandbox.RuntimeMicrosandbox), sandbox.RunSpec{Image: "agentic-claude", DryRun: true})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude:abc123", *ran)
		assert.False(t, dockerRan)
	})

	t.Run("a spec relying on a feature the runtime lacks never reaches it", func(t *testing.T) {
		// Arrange
		m := &fakeMicrosandbox{}
		ran := stubRunImage(m)
		rs := sandbox.RunSpec{Image: "agentic-claude", Egress: sandbox.EgressPolicy{Mode: sandbox.EgressMonitor}}

		// Act
		err := newTestService(&fakeDocker{inspectImage: inspectImageID("abc123")}, m).Run(runtimeRequest(sandbox.RuntimeMicrosandbox), rs)

		// Assert
		assert.ErrorContains(t, err, "can't enforce proxy monitor mode")
		assert.Empty(t, *ran)
	})

	t.Run("mounts that changed since the spec was built never reach the runtime", func(t *testing.T) {
		// Arrange
		ws := chdirSymlinkWorkspace(t)
		m := &fakeMicrosandbox{}
		ran := stubRunImage(m)
		rs := sandbox.RunSpec{Image: "agentic-claude", ToolHome: t.TempDir(), Volumes: []string{"$PWD/link:/data:ro"}}

		// Act
		err := newTestService(&fakeDocker{inspectImage: inspectImageID("abc123")}, m).Run(runtimeRequest(sandbox.RuntimeMicrosandbox), rs)

		// Assert
		assert.ErrorContains(t, err, "goes through workspace symlink "+filepath.Join(ws.dir, "link"))
		assert.Empty(t, *ran)
	})

	t.Run("unset runtime runs on docker", func(t *testing.T) {
		// Arrange
		dockerRan := false
		svc := newTestService(&fakeDocker{runTool: func(docker.Sidecars) error {
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
		svc := newTestService(&fakeDocker{}, nil)

		// Act
		err := svc.Run(runtimeRequest(sandbox.Runtime("podman")), sandbox.RunSpec{})

		// Assert
		assert.ErrorContains(t, err, `unknown runtime "podman"`)
	})
}

func Test_Service_runner(t *testing.T) {
	svc := newTestService(&fakeDocker{}, &fakeMicrosandbox{})

	t.Run("docker and the empty runtime get the docker runner", func(t *testing.T) {
		// Act
		docker, dockerErr := svc.runner(sandbox.RuntimeDocker, "")
		empty, emptyErr := svc.runner("", "")

		// Assert
		require.NoError(t, dockerErr)
		require.NoError(t, emptyErr)
		assert.Equal(t, dockerRunner{docker: svc.docker, sidecarImages: svc.sidecarImages}, docker)
		assert.Equal(t, dockerRunner{docker: svc.docker, sidecarImages: svc.sidecarImages}, empty)
	})

	t.Run("microsandbox gets the microsandbox runner", func(t *testing.T) {
		// Act
		r, err := svc.runner(sandbox.RuntimeMicrosandbox, "")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, microsandboxRunner{docker: svc.docker, microsandbox: svc.microsandbox, sidecarImages: svc.sidecarImages}, r)
	})

	t.Run("microsandbox with the agentic backend gets the agentic-proxy runner", func(t *testing.T) {
		// Act
		r, err := svc.runner(sandbox.RuntimeMicrosandbox, config.BackendAgentic)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, microsandboxRunner{docker: svc.docker, microsandbox: svc.microsandbox, sidecarImages: svc.sidecarImages, agenticProxy: true}, r)
	})

	t.Run("every parseable runtime has a runner", func(t *testing.T) {
		for _, runtime := range sandbox.Runtimes {
			// Act
			_, err := svc.runner(runtime, "")

			// Assert
			assert.NoError(t, err, runtime)
		}
	})

	t.Run("an unknown runtime is an error", func(t *testing.T) {
		// Act
		_, err := svc.runner("podman", "")

		// Assert
		assert.ErrorContains(t, err, `unknown runtime "podman"`)
	})
}

func Test_Service_capabilities(t *testing.T) {
	dockerCaps := sandbox.Capabilities{Features: sandbox.Features{Proxy: true}}
	msbCaps := sandbox.Capabilities{Features: sandbox.Features{CredentialInjection: true}}
	svc := newTestService(
		&fakeDocker{capabilities: func() sandbox.Capabilities { return dockerCaps }},
		&fakeMicrosandbox{capabilities: func() sandbox.Capabilities { return msbCaps }},
	)

	t.Run("docker and the empty runtime ask docker", func(t *testing.T) {
		// Act
		docker := svc.capabilities(sandbox.RuntimeDocker, "")
		empty := svc.capabilities("", "")

		// Assert
		assert.Equal(t, dockerCaps, docker)
		assert.Equal(t, dockerCaps, empty)
	})

	t.Run("microsandbox asks msb", func(t *testing.T) {
		// Act
		caps := svc.capabilities(sandbox.RuntimeMicrosandbox, "")

		// Assert
		assert.Equal(t, msbCaps, caps)
	})

	t.Run("an unknown runtime offers nothing", func(t *testing.T) {
		// Act
		caps := svc.capabilities("podman", "")

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
		err := newTestService(&fakeDocker{}, nil).checkRuntime(req)

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
		err := newTestService(&fakeDocker{}, m).checkRuntime(req)

		// Assert
		assert.ErrorContains(t, err, "does not support proxy monitor mode (--proxy-monitor)")
		assert.False(t, checked)
	})

	t.Run("an unknown runtime is an error", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Flags.Runtime = "podman"

		// Act
		err := newTestService(&fakeDocker{}, nil).checkRuntime(req)

		// Assert
		assert.ErrorContains(t, err, `unknown runtime "podman"`)
	})

	t.Run("microsandbox runs its own checks", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Flags.Runtime = sandbox.RuntimeMicrosandbox
		m := &fakeMicrosandbox{available: func() error { return errors.New("msb not found on PATH") }}

		// Act
		err := newTestService(&fakeDocker{}, m).checkRuntime(req)

		// Assert
		assert.EqualError(t, err, "msb not found on PATH")
	})
}

func Test_Service_Run_watchdog(t *testing.T) {
	t.Run("spawns the watchdog for the run before the runtime starts anything", func(t *testing.T) {
		// Arrange
		w := &fakeWatchdog{}
		var spawnedFirst bool
		d := &fakeDocker{startSidecars: func(rs sandbox.RunSpec, _ string) (docker.Sidecars, error) {
			spawnedFirst = len(w.spawned) == 1 && w.spawned[0] == rs.RunID
			return docker.Sidecars{}, nil
		}}
		svc := New(d, &fakeMicrosandbox{}, &fakeToolUpdates{}, &fakeSidecarImages{}, w)

		// Act
		err := svc.Run(runtimeRequest(sandbox.RuntimeDocker), sandbox.RunSpec{Image: "agentic-claude"})

		// Assert
		require.NoError(t, err)
		assert.True(t, spawnedFirst)
	})

	t.Run("a dry run spawns none", func(t *testing.T) {
		// Arrange
		w := &fakeWatchdog{}
		svc := New(&fakeDocker{}, &fakeMicrosandbox{}, &fakeToolUpdates{}, &fakeSidecarImages{}, w)

		// Act
		err := svc.Run(runtimeRequest(sandbox.RuntimeDocker), sandbox.RunSpec{Image: "agentic-claude", DryRun: true})

		// Assert
		require.NoError(t, err)
		assert.Empty(t, w.spawned)
	})

	t.Run("a failed spawn only warns", func(t *testing.T) {
		// Arrange
		errOut := iostreamstest.StubErr(t)
		ran := false
		d := &fakeDocker{runTool: func(docker.Sidecars) error {
			ran = true
			return nil
		}}
		w := &fakeWatchdog{err: errors.New("stub: spawn failed")}
		svc := New(d, &fakeMicrosandbox{}, &fakeToolUpdates{}, &fakeSidecarImages{}, w)

		// Act
		err := svc.Run(runtimeRequest(sandbox.RuntimeDocker), sandbox.RunSpec{Image: "agentic-claude"})

		// Assert
		require.NoError(t, err)
		assert.True(t, ran)
		assert.Contains(t, errOut.String(), "stub: spawn failed")
	})
}

func Test_Service_Teardown(t *testing.T) {
	const id sandbox.RunID = "0123456789ab"

	for _, tc := range []struct {
		name        string
		runtime     sandbox.Runtime
		wantDocker  bool
		wantMsb     bool
		wantErrText string
	}{
		{
			name:       "docker tears down its containers",
			runtime:    sandbox.RuntimeDocker,
			wantDocker: true,
		},
		{
			name:       "microsandbox tears down its VM and its published proxy",
			runtime:    sandbox.RuntimeMicrosandbox,
			wantDocker: true,
			wantMsb:    true,
		},
		{
			name:        "an unknown runtime is an error",
			runtime:     "example",
			wantErrText: `unknown runtime "example"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			var dockerHome string
			var dockerID, msbID sandbox.RunID
			d := &fakeDocker{teardown: func(toolHome string, got sandbox.RunID) error {
				dockerHome, dockerID = toolHome, got
				return nil
			}}
			m := &fakeMicrosandbox{teardown: func(got sandbox.RunID) error {
				msbID = got
				return nil
			}}

			// Act
			err := newTestService(d, m).Teardown(tc.runtime, "/example.test/home", id)

			// Assert
			if tc.wantErrText != "" {
				require.EqualError(t, err, tc.wantErrText)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantDocker, dockerID == id)
			assert.Equal(t, tc.wantDocker, dockerHome == "/example.test/home")
			assert.Equal(t, tc.wantMsb, msbID == id)
		})
	}

	t.Run("microsandbox joins both errors", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{teardown: func(string, sandbox.RunID) error { return errors.New("stub: docker failed") }}
		m := &fakeMicrosandbox{teardown: func(sandbox.RunID) error { return errors.New("stub: msb failed") }}

		// Act
		err := newTestService(d, m).Teardown(sandbox.RuntimeMicrosandbox, "/example.test/home", id)

		// Assert
		require.ErrorContains(t, err, "stub: docker failed")
		require.ErrorContains(t, err, "stub: msb failed")
	})
}
