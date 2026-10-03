package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartDind(t *testing.T) {
	stubDindReadyTimeout(t)
	stubHostUserGroup(t, "1234:5678")

	t.Run("creates own network and a hardened rootless sidecar", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t)
		rs := RunSpec{ToolHome: t.TempDir(), DindImage: "docker:29-dind-rootless", PidsLimit: "1024", CPUs: "4", Memory: "4g"}

		// Act
		handle, err := startDind(rs)

		// Assert
		require.NoError(t, err)
		calls := get()
		assert.Equal(t, []string{"network", "create", "--label=project=agentic-cli", handle.network}, findCall(calls, "network", "create"))

		runArgs := findCall(calls, "run")
		assert.Contains(t, runArgs, "--detach")
		assert.Contains(t, runArgs, "--read-only")
		assert.Contains(t, runArgs, "--network="+handle.network)
		assert.Contains(t, runArgs, "--network-alias=agentic-docker")
		assert.Contains(t, runArgs, "--cap-drop=ALL")
		assert.Contains(t, runArgs, "--cap-add=SYS_ADMIN")
		assert.Contains(t, runArgs, "--security-opt=seccomp="+filepath.Join(handle.runDir, "seccomp.json"))
		assert.NotContains(t, runArgs, "--security-opt=seccomp=unconfined")
		assert.FileExists(t, filepath.Join(handle.runDir, "seccomp.json"))
		assert.Contains(t, runArgs, "--device=/dev/net/tun")
		assert.Contains(t, runArgs, "--pids-limit=1024")
		assert.Contains(t, runArgs, "--env=DOCKER_TLS_CERTDIR=/certs")
		assert.Contains(t, runArgs, "--volume="+filepath.Join(handle.runDir, "server")+":/certs/server:ro")
		assert.NotContains(t, runArgs, "--privileged")
		assert.Contains(t, runArgs, "--user=1234:5678", "sidecar runs as the host user, like the tool container")
		assert.Contains(t, runArgs, "--env=HOME=/home/rootless")
		assert.Contains(t, runArgs, "--volume="+filepath.Join(handle.runDir, "etc", "subuid")+":/etc/subuid:ro")
		assert.FileExists(t, filepath.Join(handle.runDir, "etc", "passwd"))
		assert.False(t, hasArgWithPrefix(runArgs, "--env=HTTP_PROXY="), "no proxy env without proxy mode")
		assert.Equal(t, "docker:29-dind-rootless", runArgs[len(runArgs)-1])

		assert.NotNil(t, findCall(calls, "exec"), "should probe dockerd before returning")
		assert.FileExists(t, filepath.Join(handle.runDir, "client", "cert.pem"))
	})

	t.Run("joins the proxy network and routes egress through the proxy", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t)
		rs := RunSpec{ToolHome: t.TempDir(), DindImage: "dind", ProxyEnabled: true, network: "agentic-proxy-abc"}

		// Act
		handle, err := startDind(rs)

		// Assert
		require.NoError(t, err)
		calls := get()
		assert.False(t, handle.ownsNetwork)
		assert.Nil(t, findCall(calls, "network", "create"))

		runArgs := findCall(calls, "run")
		assert.Contains(t, runArgs, "--network=agentic-proxy-abc")
		assert.Contains(t, runArgs, "--env=HTTPS_PROXY=http://agentic-proxy:3128")
		assert.FileExists(t, filepath.Join(handle.runDir, "config", "config.json"), "inner containers and builds should get the proxy via the CLI config")
	})

	t.Run("tears everything down when dockerd never becomes ready", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, "exec")
		toolHome := t.TempDir()
		rs := RunSpec{ToolHome: toolHome, DindImage: "dind"}

		// Act
		_, err := startDind(rs)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "did not become ready")
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
		_, err := startDind(rs)

		// Assert
		require.Error(t, err)
		assert.NotNil(t, findCall(get(), "network", "rm"))
		entries, _ := os.ReadDir(filepath.Join(toolHome, dindDirName))
		assert.Empty(t, entries)
	})
}

func TestDindHandleStop(t *testing.T) {
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

func TestDindHandleToolArgs(t *testing.T) {
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
	err := SweepDindResources()

	// Assert
	require.NoError(t, err)
	require.Len(t, calls, 4)
	assert.Contains(t, calls[0], "--filter=name=agentic-docker")
	assert.Equal(t, []string{"rm", "--force", "--volumes", "id1"}, calls[1])
	assert.Contains(t, calls[2], "--filter=name=agentic-dind")
	assert.Equal(t, []string{"network", "rm", "id1"}, calls[3])
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
