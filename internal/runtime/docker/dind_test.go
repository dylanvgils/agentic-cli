package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/platform"
	"github.com/dylanvgils/agentic-cli/internal/runtime/dind"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_startDind(t *testing.T) {
	client := newTestClient()

	stubDindReadyTimeout(t, client)

	t.Run("joins the run network with a clear address pool, writes run files and probes the sidecar", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client)
		rs := toolRun{
			RunSpec: sandbox.RunSpec{
				RunID:    testRunID,
				User:     &testUser,
				ToolHome: t.TempDir(),
				Dind:     sandbox.DindSpec{},
			},
			container: "agentic-claude-abc",
			network:   "agentic-run-abc",
		}

		// Act
		handle, err := client.startDind(rs, nil)

		// Assert
		require.NoError(t, err)
		calls := get()
		assert.Nil(t, findCall(calls, "network", "create"), "setupSidecars owns the run network")
		assert.Contains(t, findCall(calls, "run"), "--network=agentic-run-abc")
		assert.NotNil(t, findCall(calls, "network", "inspect", arg("format", subnetsFormat), "agentic-run-abc"))
		assert.Contains(t, findCall(calls, "run"), "--default-address-pool=base=10.200.0.0/16,size=24")
		assert.NotNil(t, findCall(calls, "exec"), "should probe dockerd before returning")
		assert.FileExists(t, filepath.Join(handle.runDir, "client", "cert.pem"))
		assert.FileExists(t, filepath.Join(handle.runDir, "seccomp.json"))
		assert.FileExists(t, filepath.Join(handle.runDir, "etc", "passwd"))
		assert.DirExists(t, filepath.Join(handle.runDir, "config"))
	})

	t.Run("fails when dockerd never becomes ready", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, client, "exec")
		rs := toolRun{
			RunSpec: sandbox.RunSpec{
				RunID:    testRunID,
				User:     &testUser,
				ToolHome: t.TempDir(),
				Dind:     sandbox.DindSpec{},
			},
		}

		// Act
		_, err := client.startDind(rs, nil)

		// Assert
		require.ErrorContains(t, err, "docker sidecar did not become ready")
	})

	t.Run("refuses when the run network's subnets can't be read", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client, "network inspect")
		rs := toolRun{
			RunSpec: sandbox.RunSpec{
				RunID:    testRunID,
				User:     &testUser,
				ToolHome: t.TempDir(),
				Dind:     sandbox.DindSpec{},
			},
			network: "agentic-run-abc",
		}

		// Act
		_, err := client.startDind(rs, nil)

		// Assert
		require.ErrorContains(t, err, "inspect agentic-run-abc subnets")
		assert.Nil(t, findCall(get(), "run"), "the sidecar must not start without a clear pool")
	})

	t.Run("fails when the sidecar fails to start", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, client, "run")
		rs := toolRun{
			RunSpec: sandbox.RunSpec{
				RunID:    testRunID,
				User:     &testUser,
				ToolHome: t.TempDir(),
				Dind:     sandbox.DindSpec{},
			},
		}

		// Act
		_, err := client.startDind(rs, nil)

		// Assert
		require.ErrorContains(t, err, "start docker sidecar")
	})
}

func Test_setupDind(t *testing.T) {
	client := newTestClient()

	stubDindReadyTimeout(t, client)

	t.Run("dry run makes no docker calls or files", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t, client)
		toolHome := t.TempDir()
		rs := toolRun{
			RunSpec: sandbox.RunSpec{
				RunID:    testRunID,
				User:     &testUser,
				ToolHome: toolHome,
				DryRun:   true,
				Dind:     sandbox.DindSpec{Enabled: true},
			},
		}

		// Act
		args, err := client.setupDind(rs, nil)

		// Assert
		require.NoError(t, err)
		assert.NotEmpty(t, args)
		assert.Empty(t, calls(), "dry run must not invoke docker")
		assert.NoDirExists(t, filepath.Join(toolHome, dindDirName), "dry run must not write certs")
	})

	t.Run("propagates startDind error", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, client, "run")
		rs := toolRun{
			RunSpec: sandbox.RunSpec{
				RunID:    testRunID,
				User:     &testUser,
				ToolHome: t.TempDir(),
				Dind:     sandbox.DindSpec{Enabled: true},
			},
		}

		// Act
		args, err := client.setupDind(rs, nil)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, args)
	})
}

func Test_dindHandle_runArgs(t *testing.T) {
	identity, err := dind.NewIdentity(platform.User{UID: 1234, GID: 5678})
	require.NoError(t, err)
	handle := dindHandle{
		identity:    identity,
		container:   "agentic-docker-abc",
		runDir:      "/home/user/.agentic/dind/abc",
		addressPool: "base=10.200.0.0/16,size=24",
	}

	t.Run("hardened rootless sidecar", func(t *testing.T) {
		// Arrange
		rs := toolRun{RunSpec: sandbox.RunSpec{
			Limits: sandbox.ResourceLimits{PidsLimit: "1024", CPUs: "4", Memory: "4g"},
			Dind:   sandbox.DindSpec{Limits: sandbox.ResourceLimits{PidsLimit: "2048", CPUs: "2", Memory: "8g"}},
		}, container: "agentic-claude-abc", network: "agentic-run-abc"}

		// Act
		args := handle.runArgs(rs)

		// Assert
		assert.Contains(t, args, "--detach")
		assert.NotContains(t, args, "--rm", "a crashed sidecar must keep its logs")
		assert.Contains(t, args, "--read-only")
		assert.Contains(t, args, "--network=agentic-run-abc")
		assert.Contains(t, args, "--network-alias=agentic-docker")
		assert.Contains(t, args, "--cap-drop=ALL")
		assert.Contains(t, args, "--cap-add=SYS_ADMIN")
		assert.Contains(t, args, "--security-opt=seccomp="+filepath.Join(handle.runDir, "seccomp.json"))
		assert.NotContains(t, args, "--security-opt=seccomp=unconfined")
		assert.NotContains(t, args, "--privileged")
		assert.Contains(t, args, "--device=/dev/net/tun")
		assert.Contains(t, args, "--pids-limit=2048", "sidecar uses its own limits, not the tool's")
		assert.Contains(t, args, "--cpus=2")
		assert.Contains(t, args, "--memory=8g")
		assert.Contains(t, args, "--label=agentic.owner=agentic-claude-abc", "an orphaned sidecar is found by its owner")
		assert.True(t, hasArgWithPrefix(args, "--label=agentic.started="))
		assert.Contains(t, args, "--user=1234:5678", "sidecar runs as the host user, like the tool container")
		assert.Contains(t, args, "--env=HOME=/home/rootless")
		assert.Contains(t, args, "--env=DOCKER_TLS_CERTDIR=/certs")
		assert.Contains(t, args, "--volume="+filepath.Join(handle.runDir, "server")+":/certs/server:ro")
		assert.Contains(t, args, "--volume="+filepath.Join(handle.runDir, "etc", "subuid")+":/etc/subuid:ro")
		assert.False(t, hasArgWithPrefix(args, "--env=HTTP_PROXY="), "no proxy env without proxy mode")
		dockerdArgs := []string{
			tools.DindImage,
			"--data-root=/home/rootless/.local/share/docker/data",
			"--default-address-pool=base=10.200.0.0/16,size=24",
		}
		assert.Equal(t, dockerdArgs, args[len(args)-3:], "dockerd must own its data-root to chmod it")
	})

	t.Run("proxy mode routes egress through the proxy", func(t *testing.T) {
		// Arrange
		rs := toolRun{RunSpec: sandbox.RunSpec{Egress: sandbox.EgressPolicy{Mode: sandbox.EgressEnforce}, Dind: sandbox.DindSpec{}}}

		// Act
		args := handle.runArgs(rs)

		// Assert
		assert.Contains(t, args, "--env=HTTPS_PROXY=http://agentic-proxy:3128")
	})
}

func Test_dindHandle_waitReady(t *testing.T) {
	client := newTestClient()

	handle := dindHandle{client: client, container: "agentic-docker-abc"}

	t.Run("returns once dockerd answers", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, client)

		// Act
		err := handle.waitReady(nil)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("times out with logs when dockerd never answers", func(t *testing.T) {
		// Arrange
		stubDindReadyTimeout(t, client)
		stubDindProbe(t, client, "true")

		// Act
		err := handle.waitReady(nil)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "did not become ready")
		assert.Contains(t, err.Error(), "failed to setup UID/GID map")
	})

	t.Run("fails fast when the sidecar exits", func(t *testing.T) {
		// Arrange
		stubDindReadyTimeoutLong(t, client)
		stubDindProbe(t, client, "false")

		// Act
		err := handle.waitReady(nil)

		// Assert
		assert.ErrorContains(t, err, "exited before becoming ready")
	})

	t.Run("stops waiting when interrupted", func(t *testing.T) {
		// Arrange
		stubDindReadyTimeoutLong(t, client)
		stubDindProbe(t, client, "true")
		interrupt := make(chan os.Signal, 1)
		interrupt <- os.Interrupt

		// Act
		err := handle.waitReady(interrupt)

		// Assert
		assert.ErrorContains(t, err, "interrupted")
	})
}

func Test_dindHandle_toolArgs(t *testing.T) {
	// Arrange
	handle := dindHandle{runDir: "/home/user/.agentic/dind/abc"}

	// Act
	args := handle.toolArgs()

	// Assert
	assert.Equal(t, []string{
		"--env=DOCKER_HOST=tcp://agentic-docker:2376",
		"--env=DOCKER_TLS_VERIFY=1",
		"--env=DOCKER_CERT_PATH=/run/secrets/docker",
		"--env=DOCKER_CONFIG=/run/docker-config",
		"--volume=" + filepath.Join("/home/user/.agentic/dind/abc", "client") + ":/run/secrets/docker:ro",
		"--volume=" + filepath.Join("/home/user/.agentic/dind/abc", "config") + ":/run/docker-config",
	}, args)
}

func Test_sweepDindRunDirs(t *testing.T) {
	client := newTestClient()

	stubDindRunDirGrace(t, client, time.Minute)

	for _, tc := range []struct {
		name       string
		live       []ownedResource
		dir        string
		age        time.Duration
		wantExists bool
	}{
		{
			name: "removes a stale dir nothing of its run is left of",
			dir:  string(testRunID),
			age:  time.Hour,
		},
		{
			name:       "keeps a dir whose run still has a resource",
			live:       []ownedResource{{name: "agentic-claude-" + string(testRunID), run: testRunID}},
			dir:        string(testRunID),
			age:        time.Hour,
			wantExists: true,
		},
		{
			name:       "keeps a dir an older version named after its live sidecar",
			live:       []ownedResource{{name: "agentic-docker-" + string(testRunID)}},
			dir:        string(testRunID),
			age:        time.Hour,
			wantExists: true,
		},
		{
			name:       "keeps a dir younger than the grace period, as a concurrent run may not have started yet",
			dir:        string(testRunID),
			wantExists: true,
		},
		{
			name:       "ignores entries that are not run ids",
			dir:        "not-a-run-id",
			age:        time.Hour,
			wantExists: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			stubDockerRunCapture(t, client)
			toolHome := t.TempDir()
			dir := makeDindRunDir(t, toolHome, tc.dir, tc.age)

			// Act
			err := client.sweepDindRunDirs(toolHome, tc.live)

			// Assert
			require.NoError(t, err)
			if tc.wantExists {
				assert.DirExists(t, dir)
			} else {
				assert.NoDirExists(t, dir)
			}
		})
	}

	t.Run("tears down the stale dir's run", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client)
		toolHome := t.TempDir()
		makeDindRunDir(t, toolHome, string(testRunID), time.Hour)

		// Act
		err := client.sweepDindRunDirs(toolHome, nil)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, findCall(get(), "network", "rm", "agentic-run-"+string(testRunID)))
	})

	t.Run("missing dind dir is a no-op without calling docker", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client)

		// Act
		err := client.sweepDindRunDirs(t.TempDir(), nil)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, get())
	})
}

func Test_workspaceVolumes(t *testing.T) {
	// Arrange
	rs := sandbox.RunSpec{
		ToolHome:      "/home/user/.agentic",
		ContainerHome: "/home/agent",
		Volumes: []string{
			"/src/project:/workspace",
			"$TOOL_HOME/claude:$CONTAINER_HOME/.claude",
			"maven-cache:/home/agent/.m2",
			"/src/project/.git:/workspace/.git:ro",
			"/src/other:/workspace-other",
		},
	}

	// Act
	result := workspaceVolumes(rs)

	// Assert - tool home and caches stay out of the sidecar; read-only overlays keep their :ro
	require.Len(t, result, 2)
	assert.True(t, strings.HasSuffix(result[0], ":/workspace"))
	assert.True(t, strings.HasSuffix(result[1], ":/workspace/.git:ro"))
}

func Test_writeDindClientConfig(t *testing.T) {
	t.Run("creates an empty writable dir without proxy", func(t *testing.T) {
		// Arrange
		dir := filepath.Join(t.TempDir(), "config")

		// Act
		err := writeDindClientConfig(dir, false)

		// Assert
		require.NoError(t, err)
		assert.DirExists(t, dir)
		assert.NoFileExists(t, filepath.Join(dir, "config.json"))
	})

	t.Run("sets cli proxies in proxy mode", func(t *testing.T) {
		// Arrange
		dir := filepath.Join(t.TempDir(), "config")

		// Act
		err := writeDindClientConfig(dir, true)

		// Assert
		require.NoError(t, err)
		content, readErr := os.ReadFile(filepath.Join(dir, "config.json"))
		require.NoError(t, readErr)
		assert.JSONEq(t, `{"proxies":{"default":{"httpProxy":"http://agentic-proxy:3128","httpsProxy":"http://agentic-proxy:3128","noProxy":"localhost,127.0.0.1"}}}`, string(content))
	})
}

func Test_newDindHandle(t *testing.T) {
	// Arrange
	client := newTestClient()
	toolHome := t.TempDir()

	// Act
	handle, err := client.newDindHandle(toolRun{
		RunSpec: sandbox.RunSpec{
			RunID:    testRunID,
			User:     &testUser,
			ToolHome: toolHome,
		},
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "agentic-docker-"+string(testRunID), handle.container)
	assert.Equal(t, filepath.Join(toolHome, dindDirName, string(testRunID)), handle.runDir)
	assert.Equal(t, dryRunAddressPoolPlaceholder, handle.addressPool, "only startDind knows the run network's subnets")
}
