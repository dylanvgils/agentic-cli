package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/dind"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_startDind(t *testing.T) {
	stubDindReadyTimeout(t)
	stubHostUserGroup(t, "1234:5678")

	t.Run("creates own network, writes run files and probes the sidecar", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t)
		rs := RunSpec{ToolHome: t.TempDir(), DindImage: "dind", container: "agentic-claude-abc"}

		// Act
		handle, err := startDind(rs, nil)

		// Assert
		require.NoError(t, err)
		calls := get()
		createArgs := findCall(calls, "network", "create")
		assert.Contains(t, createArgs, "--label=project=agentic-cli")
		assert.Contains(t, createArgs, "--label=agentic.owner=agentic-claude-abc", "the network is swept with its owner")
		assert.Equal(t, handle.network, createArgs[len(createArgs)-1])
		assert.NotNil(t, findCall(calls, "run"))
		assert.NotNil(t, findCall(calls, "exec"), "should probe dockerd before returning")
		assert.FileExists(t, filepath.Join(handle.runDir, "client", "cert.pem"))
		assert.FileExists(t, filepath.Join(handle.runDir, "seccomp.json"))
		assert.FileExists(t, filepath.Join(handle.runDir, "etc", "passwd"))
		assert.DirExists(t, filepath.Join(handle.runDir, "config"))
	})

	t.Run("joins the proxy network instead of creating one", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t)
		rs := RunSpec{ToolHome: t.TempDir(), DindImage: "dind", ProxyEnabled: true, network: "agentic-proxy-abc"}

		// Act
		handle, err := startDind(rs, nil)

		// Assert
		require.NoError(t, err)
		calls := get()
		assert.False(t, handle.ownsNetwork)
		assert.Nil(t, findCall(calls, "network", "create"))
		assert.Contains(t, findCall(calls, "run"), "--network=agentic-proxy-abc")
	})

	t.Run("tears everything down when dockerd never becomes ready", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, "exec")
		toolHome := t.TempDir()
		rs := RunSpec{ToolHome: toolHome, DindImage: "dind"}

		// Act
		_, err := startDind(rs, nil)

		// Assert
		require.Error(t, err)
		calls := get()
		assert.NotNil(t, findCall(calls, "rm", "--force", "--volumes"))
		assert.NotNil(t, findCall(calls, "network", "rm"))
		entries, _ := os.ReadDir(filepath.Join(toolHome, dindDirName))
		assert.Empty(t, entries, "cert dir should be removed")
	})

	t.Run("removes network and certs when sidecar fails to start", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, "run")
		toolHome := t.TempDir()
		rs := RunSpec{ToolHome: toolHome, DindImage: "dind"}

		// Act
		_, err := startDind(rs, nil)

		// Assert
		require.Error(t, err)
		assert.NotNil(t, findCall(get(), "network", "rm"))
		entries, _ := os.ReadDir(filepath.Join(toolHome, dindDirName))
		assert.Empty(t, entries)
	})
}

func Test_setupDind(t *testing.T) {
	stubDindReadyTimeout(t)
	stubHostUserGroup(t, "1234:5678")

	t.Run("dry run sets own network without docker calls or files", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t)
		toolHome := t.TempDir()
		rs := RunSpec{ToolHome: toolHome, DindImage: "dind", DindEnabled: true, DryRun: true}

		// Act
		args, cleanup, err := setupDind(&rs)

		// Assert
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(rs.network, dindNetworkPrefix+"-"))
		assert.NotEmpty(t, args)
		assert.Empty(t, calls(), "dry run must not invoke docker")
		assert.NoDirExists(t, filepath.Join(toolHome, dindDirName), "dry run must not write certs")
		require.NotPanics(t, cleanup)
	})

	t.Run("dry run keeps proxy network", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t)
		rs := RunSpec{ToolHome: t.TempDir(), DindImage: "dind", DindEnabled: true, DryRun: true, ProxyEnabled: true, network: "agentic-proxy-abc"}

		// Act
		_, _, err := setupDind(&rs)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-proxy-abc", rs.network)
	})

	t.Run("cleanup removes the sidecar", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t)
		toolHome := t.TempDir()
		rs := RunSpec{ToolHome: toolHome, DindImage: "dind", DindEnabled: true}

		// Act
		_, cleanup, err := setupDind(&rs)
		cleanup()

		// Assert
		require.NoError(t, err)
		last := calls()[len(calls())-1]
		assert.Equal(t, []string{"network", "rm", rs.network}, last.args)
		entries, _ := os.ReadDir(filepath.Join(toolHome, dindDirName))
		assert.Empty(t, entries, "per-run certs should be removed")
	})

	t.Run("propagates startDind error", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, "run")
		rs := RunSpec{ToolHome: t.TempDir(), DindImage: "dind", DindEnabled: true}

		// Act
		args, cleanup, err := setupDind(&rs)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, args)
		assert.Nil(t, cleanup)
	})
}

func Test_dindHandle_runArgs(t *testing.T) {
	identity, err := dind.NewIdentity("1234:5678")
	require.NoError(t, err)
	handle := dindHandle{identity: identity, container: "agentic-docker-abc", network: "agentic-dind-abc", runDir: "/home/user/.agentic/dind/abc"}

	t.Run("hardened rootless sidecar", func(t *testing.T) {
		// Arrange
		rs := RunSpec{
			DindImage: "docker:29-dind-rootless", PidsLimit: "1024", CPUs: "4", Memory: "4g",
			DindPidsLimit: "2048", DindCPUs: "2", DindMemory: "8g", container: "agentic-claude-abc",
		}

		// Act
		args := handle.runArgs(rs)

		// Assert
		assert.Contains(t, args, "--detach")
		assert.NotContains(t, args, "--rm", "a crashed sidecar must keep its logs")
		assert.Contains(t, args, "--read-only")
		assert.Contains(t, args, "--network=agentic-dind-abc")
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
		assert.Equal(t, []string{"docker:29-dind-rootless", "--data-root=/home/rootless/.local/share/docker/data"}, args[len(args)-2:], "dockerd must own its data-root to chmod it")
	})

	t.Run("proxy mode routes egress through the proxy", func(t *testing.T) {
		// Arrange
		rs := RunSpec{DindImage: "dind", ProxyEnabled: true}

		// Act
		args := handle.runArgs(rs)

		// Assert
		assert.Contains(t, args, "--env=HTTPS_PROXY=http://agentic-proxy:3128")
	})
}

func Test_dindHandle_waitReady(t *testing.T) {
	handle := dindHandle{container: "agentic-docker-abc"}

	t.Run("returns once dockerd answers", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t)

		// Act
		err := handle.waitReady(nil)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("times out with logs when dockerd never answers", func(t *testing.T) {
		// Arrange
		stubDindReadyTimeout(t)
		stubDindProbe(t, "true")

		// Act
		err := handle.waitReady(nil)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "did not become ready")
		assert.Contains(t, err.Error(), "failed to setup UID/GID map")
	})

	t.Run("fails fast when the sidecar exits", func(t *testing.T) {
		// Arrange
		stubDindReadyTimeoutLong(t)
		stubDindProbe(t, "false")

		// Act
		err := handle.waitReady(nil)

		// Assert
		assert.ErrorContains(t, err, "exited before becoming ready")
	})

	t.Run("stops waiting when interrupted", func(t *testing.T) {
		// Arrange
		stubDindReadyTimeoutLong(t)
		stubDindProbe(t, "true")
		interrupt := make(chan os.Signal, 1)
		interrupt <- os.Interrupt

		// Act
		err := handle.waitReady(interrupt)

		// Assert
		assert.ErrorContains(t, err, "interrupted")
	})
}

func Test_dindHandle_Stop(t *testing.T) {
	t.Run("removes owned network", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t)
		runDir := filepath.Join(t.TempDir(), "abc")
		require.NoError(t, os.MkdirAll(runDir, 0o700))
		handle := dindHandle{container: "agentic-docker-abc", network: "agentic-dind-abc", ownsNetwork: true, runDir: runDir}

		// Act
		handle.Stop()

		// Assert
		calls := get()
		require.Len(t, calls, 2)
		assert.Equal(t, []string{"rm", "--force", "--volumes", "agentic-docker-abc"}, calls[0].args)
		assert.Equal(t, []string{"network", "rm", "agentic-dind-abc"}, calls[1].args)
		assert.NoDirExists(t, runDir)
	})

	t.Run("leaves a shared proxy network alone", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t)
		handle := dindHandle{container: "agentic-docker-abc", network: "agentic-proxy-abc", runDir: t.TempDir()}

		// Act
		handle.Stop()

		// Assert
		assert.Nil(t, findCall(get(), "network", "rm"))
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

func TestSweepDindResources(t *testing.T) {
	// Arrange
	var calls [][]string
	stubDockerRun(t, func(args ...string) (string, error) {
		calls = append(calls, args)
		if args[0] == "ps" || (args[0] == "network" && args[1] == "ls") {
			return "id1\n", nil
		}
		return "", nil
	})

	// Act
	err := SweepDindResources(t.TempDir())

	// Assert
	require.NoError(t, err)
	require.Len(t, calls, 4)
	assert.Contains(t, calls[0], "--filter=name=agentic-docker")
	assert.Equal(t, []string{"rm", "--force", "--volumes", "id1"}, calls[1])
	assert.Contains(t, calls[2], "--filter=name=agentic-dind")
	assert.Equal(t, []string{"network", "rm", "id1"}, calls[3])
}

func Test_sweepDindRunDirs(t *testing.T) {
	stubDindRunDirGrace(t, time.Minute)

	t.Run("removes a stale dir whose sidecar is gone", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", nil)
		toolHome := t.TempDir()
		dir := makeDindRunDir(t, toolHome, "0123456789ab", time.Hour)

		// Act
		err := sweepDindRunDirs(toolHome)

		// Assert
		require.NoError(t, err)
		assert.NoDirExists(t, dir)
	})

	t.Run("keeps a dir whose sidecar still exists", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "agentic-docker-0123456789ab\n", nil)
		toolHome := t.TempDir()
		dir := makeDindRunDir(t, toolHome, "0123456789ab", time.Hour)

		// Act
		err := sweepDindRunDirs(toolHome)

		// Assert
		require.NoError(t, err)
		assert.DirExists(t, dir)
	})

	t.Run("keeps a dir younger than the grace period", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", nil)
		toolHome := t.TempDir()
		dir := makeDindRunDir(t, toolHome, "0123456789ab", 0)

		// Act
		err := sweepDindRunDirs(toolHome)

		// Assert
		require.NoError(t, err)
		assert.DirExists(t, dir, "a concurrent run may not have started its sidecar yet")
	})

	t.Run("ignores entries that are not run ids", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", nil)
		toolHome := t.TempDir()
		dir := makeDindRunDir(t, toolHome, "not-a-run-id", time.Hour)

		// Act
		err := sweepDindRunDirs(toolHome)

		// Assert
		require.NoError(t, err)
		assert.DirExists(t, dir)
	})

	t.Run("missing dind dir is a no-op without calling docker", func(t *testing.T) {
		// Arrange
		called := false
		stubDockerRun(t, func(...string) (string, error) {
			called = true
			return "", nil
		})

		// Act
		err := sweepDindRunDirs(t.TempDir())

		// Assert
		require.NoError(t, err)
		assert.False(t, called)
	})

	t.Run("docker error propagates and keeps dirs", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", fmt.Errorf("daemon down"))
		toolHome := t.TempDir()
		dir := makeDindRunDir(t, toolHome, "0123456789ab", time.Hour)

		// Act
		err := sweepDindRunDirs(toolHome)

		// Assert
		require.Error(t, err)
		assert.DirExists(t, dir, "without the live list a running sidecar's certs could be deleted")
	})
}

func TestIsReservedDindEnvName(t *testing.T) {
	t.Run("docker client vars are reserved", func(t *testing.T) {
		for _, key := range []string{"DOCKER_HOST", "DOCKER_TLS_VERIFY", "DOCKER_CERT_PATH", "DOCKER_CONTEXT", "DOCKER_CONFIG"} {
			// Act
			result := IsReservedDindEnvName(key)

			// Assert
			assert.True(t, result, key)
		}
	})

	t.Run("unrelated var is not reserved", func(t *testing.T) {
		// Act
		result := IsReservedDindEnvName("DOCKER_BUILDKIT")

		// Assert
		assert.False(t, result)
	})
}

func Test_workspaceVolumes(t *testing.T) {
	// Arrange
	rs := RunSpec{
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
	stubHostUserGroup(t, "1000:1000")

	// Act
	handle, err := newDindHandle(RunSpec{ToolHome: t.TempDir()})

	// Assert
	require.NoError(t, err)
	assert.True(t, handle.ownsNetwork)
	assert.True(t, strings.HasPrefix(handle.network, "agentic-dind-"))
}
