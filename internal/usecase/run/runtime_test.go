package run

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Apply(t *testing.T) {
	t.Run("runs the watched run's spec on its runner, then ends the run", func(t *testing.T) {
		// Arrange
		var ranSpec sandbox.RunSpec
		var dirDuringRun bool
		d := &fakeDocker{startSidecars: func(rs sandbox.RunSpec, _ string) (docker.Sidecars, error) {
			ranSpec = rs
			_, err := os.Stat(sandbox.RunTempDir(rs.RunID))
			dirDuringRun = err == nil
			return docker.Sidecars{}, nil
		}}
		w := &fakeWatchdog{}
		svc := New(d, nil, &fakeToolUpdates{}, &fakeSidecarImages{}, w, &fakeCommandLog{})
		plan := newPlan(t, svc, newTestRunConfig(t))

		// Act
		err := svc.Apply(plan)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude", ranSpec.Image)
		require.Len(t, w.spawned, 1)
		assert.Equal(t, w.spawned[0].ID, ranSpec.RunID)
		assert.True(t, dirDuringRun)
		assert.NoDirExists(t, sandbox.RunTempDir(ranSpec.RunID))
	})

	t.Run("keeps the command log open in the run's log dir while the runner runs", func(t *testing.T) {
		// Arrange
		l := &fakeCommandLog{}
		closedDuringRun := -1
		d := &fakeDocker{startSidecars: func(sandbox.RunSpec, string) (docker.Sidecars, error) {
			closedDuringRun = l.closed
			return docker.Sidecars{}, nil
		}}
		svc := New(d, nil, &fakeToolUpdates{}, &fakeSidecarImages{}, &fakeWatchdog{}, l)
		plan := newPlan(t, svc, newTestRunConfig(t))

		// Act
		err := svc.Apply(plan)

		// Assert
		require.NoError(t, err)
		require.Len(t, l.opened, 1)
		assert.Equal(t, commandLogFileName, filepath.Base(l.opened[0]))
		assert.DirExists(t, filepath.Dir(l.opened[0]))
		assert.Equal(t, 0, closedDuringRun)
		assert.Equal(t, 1, l.closed)
	})

	t.Run("a failed build ends the run", func(t *testing.T) {
		// Arrange
		w := &fakeWatchdog{}
		svc := New(&fakeDocker{}, nil, &fakeToolUpdates{}, &fakeSidecarImages{}, w, &fakeCommandLog{})
		c := newTestRunConfig(t)
		c.Env = []string{"TOOL_HOME=nope"}
		plan := newPlan(t, svc, c)

		// Act
		err := svc.Apply(plan)

		// Assert
		require.Error(t, err)
		require.Len(t, w.spawned, 1)
		assert.NoDirExists(t, sandbox.RunTempDir(w.spawned[0].ID))
	})

	t.Run("a spec relying on a feature the runtime lacks never reaches it and ends the run", func(t *testing.T) {
		// Arrange
		m := &fakeMicrosandbox{}
		ran := stubRunImage(m)
		w := &fakeWatchdog{}
		svc := New(&fakeDocker{inspectImage: inspectImageID("abc123")}, m, &fakeToolUpdates{}, &fakeSidecarImages{}, w, &fakeCommandLog{})
		c := newTestRunConfig(t)
		c.Runtime.Name = sandbox.RuntimeMicrosandbox
		c.Egress.Mode = sandbox.EgressMonitor
		plan := newPlan(t, svc, c)

		// Act
		err := svc.Apply(plan)

		// Assert
		assert.ErrorContains(t, err, "can't enforce proxy monitor mode")
		assert.Empty(t, *ran)
		require.Len(t, w.spawned, 1)
		assert.NoDirExists(t, sandbox.RunTempDir(w.spawned[0].ID))
	})
}

func TestService_DryRun(t *testing.T) {
	t.Run("prints the unwatched run's spec through its runner without a log, then ends the run", func(t *testing.T) {
		// Arrange
		var printed sandbox.RunSpec
		d := &fakeDocker{dryRun: func(rs sandbox.RunSpec, _ string) error {
			printed = rs
			return nil
		}}
		w := &fakeWatchdog{}
		l := &fakeCommandLog{}
		svc := New(d, nil, &fakeToolUpdates{}, &fakeSidecarImages{}, w, l)
		c := newTestRunConfig(t)
		plan := newPlan(t, svc, c)

		// Act
		err := svc.DryRun(plan)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude", printed.Image)
		assert.Empty(t, w.spawned)
		assert.NoDirExists(t, sandbox.RunTempDir(printed.RunID))
		assert.Empty(t, l.opened)
		assert.NoDirExists(t, filepath.Join(c.Homes.Tool, logging.DirName))
	})

	t.Run("a spec relying on a feature the runtime lacks is refused", func(t *testing.T) {
		// Arrange
		m := &fakeMicrosandbox{dryRun: func(sandbox.RunSpec) error {
			t.Fatal("printed the spec")
			return nil
		}}
		svc := newTestService(&fakeDocker{}, m)
		c := newTestRunConfig(t)
		c.Runtime.Name = sandbox.RuntimeMicrosandbox
		c.Egress.Mode = sandbox.EgressMonitor
		plan := newPlan(t, svc, c)

		// Act
		err := svc.DryRun(plan)

		// Assert
		assert.ErrorContains(t, err, "can't enforce proxy monitor mode")
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

func Test_Service_checkRuntime(t *testing.T) {
	t.Run("a missing capability stops before the msb checks", func(t *testing.T) {
		// Arrange
		req := newResolveRequest(t)
		req.Flags.Runtime = sandbox.RuntimeMicrosandbox
		req.Flags.Proxy.MonitorFlag = true
		checked := false
		m := &fakeMicrosandbox{available: func() error {
			checked = true
			return nil
		}}

		// Act
		_, err := newTestService(&fakeDocker{}, m).checkRuntime(req, chooseRuntime(req.Flags, req.Project.RC))

		// Assert
		assert.ErrorContains(t, err, "does not support proxy monitor mode (--proxy-monitor)")
		assert.False(t, checked)
	})

	t.Run("an unknown runtime is an error", func(t *testing.T) {
		// Arrange
		req := newResolveRequest(t)
		req.Flags.Runtime = "podman"

		// Act
		_, err := newTestService(&fakeDocker{}, nil).checkRuntime(req, chooseRuntime(req.Flags, req.Project.RC))

		// Assert
		assert.ErrorContains(t, err, `unknown runtime "podman"`)
	})

	t.Run("microsandbox runs its own checks", func(t *testing.T) {
		// Arrange
		req := newResolveRequest(t)
		req.Flags.Runtime = sandbox.RuntimeMicrosandbox
		m := &fakeMicrosandbox{available: func() error { return errors.New("msb not found on PATH") }}

		// Act
		_, err := newTestService(&fakeDocker{}, m).checkRuntime(req, chooseRuntime(req.Flags, req.Project.RC))

		// Assert
		assert.EqualError(t, err, "msb not found on PATH")
	})
}

func Test_Service_Teardown(t *testing.T) {
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
			stubRunTempDir(t)
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
			run := sandbox.RunRef{
				ID:       testRunID,
				Runtime:  tc.runtime,
				Tool:     "claude",
				ToolHome: "/example.test/home",
			}

			// Act
			err := newTestService(d, m).Teardown(run)

			// Assert
			if tc.wantErrText != "" {
				require.EqualError(t, err, tc.wantErrText)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantDocker, dockerID == testRunID)
			assert.Equal(t, tc.wantDocker, dockerHome == "/example.test/home")
			assert.Equal(t, tc.wantMsb, msbID == testRunID)
			assert.NoDirExists(t, sandbox.RunTempDir(testRunID))
		})
	}

	t.Run("microsandbox joins both errors", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{teardown: func(string, sandbox.RunID) error { return errors.New("stub: docker failed") }}
		m := &fakeMicrosandbox{teardown: func(sandbox.RunID) error { return errors.New("stub: msb failed") }}
		run := sandbox.RunRef{
			ID:       testRunID,
			Runtime:  sandbox.RuntimeMicrosandbox,
			ToolHome: "/example.test/home",
		}

		// Act
		err := newTestService(d, m).Teardown(run)

		// Assert
		require.ErrorContains(t, err, "stub: docker failed")
		require.ErrorContains(t, err, "stub: msb failed")
	})

	t.Run("saves a leftover instructions snapshot to the tool's file once the runtime is torn down", func(t *testing.T) {
		// Arrange
		stubRunTempDir(t)
		toolHome := t.TempDir()
		require.NoError(t, tools.Configs["claude"].Runtime.Setup(toolHome))
		c := newTestRunConfig(t)
		c.Homes.Tool = toolHome
		mountSpec, err := newTestService(&fakeDocker{}, nil).prepareInstructions(c, testRunID, "generated block")
		require.NoError(t, err)
		snapshotPath := sandbox.HostPart(mountSpec)
		var snapshotAtTeardown bool
		d := &fakeDocker{teardown: func(string, sandbox.RunID) error {
			_, statErr := os.Stat(snapshotPath)
			snapshotAtTeardown = statErr == nil
			return nil
		}}
		run := sandbox.RunRef{
			ID:       testRunID,
			Runtime:  sandbox.RuntimeDocker,
			Tool:     "claude",
			ToolHome: toolHome,
		}

		// Act
		err = newTestService(d, nil).Teardown(run)

		// Assert
		require.NoError(t, err)
		assert.True(t, snapshotAtTeardown)
		assert.NoDirExists(t, sandbox.RunTempDir(testRunID))
	})
}

func Test_Service_startRun(t *testing.T) {
	t.Run("spawns the run's watchdog before creating its temp dir", func(t *testing.T) {
		// Arrange
		t.Setenv("TMPDIR", t.TempDir())
		c := newTestRunConfig(t)
		c.Runtime.Name = sandbox.RuntimeDocker
		var dirAtSpawn bool
		w := &fakeWatchdog{onSpawn: func(run sandbox.RunRef) {
			_, err := os.Stat(sandbox.RunTempDir(run.ID))
			dirAtSpawn = err == nil
		}}
		svc := New(&fakeDocker{}, nil, &fakeToolUpdates{}, &fakeSidecarImages{}, w, &fakeCommandLog{})

		// Act
		run, err := svc.startRun(c, true)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.RunRef{
			ID:       run.ID,
			Runtime:  sandbox.RuntimeDocker,
			Tool:     "claude",
			ToolHome: c.Homes.Tool,
		}, run)
		assert.Equal(t, []sandbox.RunRef{run}, w.spawned)
		assert.False(t, dirAtSpawn)
		assert.DirExists(t, sandbox.RunTempDir(run.ID))
	})

	t.Run("an unwatched run spawns none", func(t *testing.T) {
		// Arrange
		t.Setenv("TMPDIR", t.TempDir())
		w := &fakeWatchdog{}
		svc := New(&fakeDocker{}, nil, &fakeToolUpdates{}, &fakeSidecarImages{}, w, &fakeCommandLog{})

		// Act
		run, err := svc.startRun(newTestRunConfig(t), false)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, w.spawned)
		assert.DirExists(t, sandbox.RunTempDir(run.ID))
	})

	t.Run("a failed spawn only warns", func(t *testing.T) {
		// Arrange
		errOut := iostreamstest.StubErr(t)
		t.Setenv("TMPDIR", t.TempDir())
		w := &fakeWatchdog{err: errors.New("stub: spawn failed")}
		svc := New(&fakeDocker{}, nil, &fakeToolUpdates{}, &fakeSidecarImages{}, w, &fakeCommandLog{})

		// Act
		run, err := svc.startRun(newTestRunConfig(t), true)

		// Assert
		require.NoError(t, err)
		assert.DirExists(t, sandbox.RunTempDir(run.ID))
		assert.Contains(t, errOut.String(), "stub: spawn failed")
	})
}
