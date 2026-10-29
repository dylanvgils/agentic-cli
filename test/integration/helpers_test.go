//go:build integration

package integration

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/stretchr/testify/require"
)

// agenticCmd returns an agentic command running in workDir with the test AGENTIC_HOME and TMPDIR.
func agenticCmd(args ...string) *exec.Cmd {
	return agenticCmdIn(workDir, args...)
}

// agenticCmdIn is agenticCmd running in dir.
func agenticCmdIn(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command(agenticBin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "AGENTIC_HOME="+agenticHome, "TMPDIR="+tmpDir)
	return cmd
}

// runningProxy returns the name of the one running proxy sidecar.
func runningProxy(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("docker", "ps", "--filter", "name=agentic-proxy-", "--format", "{{.Names}}").Output()
	require.NoError(t, err)
	names := strings.Fields(string(out))
	require.Len(t, names, 1, "expected one running proxy")
	return names[0]
}

// copyFromProxy copies file from the proxy's credentials dir, failing when it doesn't exist.
func copyFromProxy(proxy, file string) error {
	return exec.Command("docker", "cp", proxy+":"+proxyCredentialsDir+"/"+file, "-").Run()
}

// curlFromEgressNet requests url through the running proxy from a container on the shared egress network the proxy also
// joins, and returns curl's output with a final status=<code> line.
func curlFromEgressNet(t *testing.T, url string) string {
	t.Helper()
	ip, err := exec.Command("docker", "inspect", "--format", `{{(index .NetworkSettings.Networks "agentic-net").IPAddress}}`, runningProxy(t)).Output()
	require.NoError(t, err)
	proxyURL := "http://" + strings.TrimSpace(string(ip)) + ":3128"

	out, err := exec.Command("docker", "run", "--rm", "--network", "agentic-net", "--entrypoint", "",
		testNamespace+"-"+testTool, "curl", "-sS", "-m", "5", "-x", proxyURL, "-w", "\nstatus=%{http_code}", url).CombinedOutput()
	require.NoError(t, err, string(out))
	return string(out)
}

// skipWithoutCgroups skips when the daemon silently ignores resource limits (e.g. rootless without systemd).
func skipWithoutCgroups(t *testing.T) {
	t.Helper()
	out, err := exec.Command("docker", "info", "--format", "{{.CgroupDriver}}").Output()
	require.NoError(t, err)
	if strings.TrimSpace(string(out)) == "none" {
		t.Skip("docker daemon runs without cgroups, so resource limits aren't enforced")
	}
}

// tempDirIn creates a dir under parent, removed on cleanup; use it over t.TempDir for anything bind-mounted.
func tempDirIn(t *testing.T, parent string) string {
	t.Helper()
	dir, err := os.MkdirTemp(parent, "itest-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// uniqueName returns a docker resource name that won't clash with real agentic resources.
func uniqueName(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 6)
	_, err := rand.Read(buf)
	require.NoError(t, err)
	return "agentic-itest-" + hex.EncodeToString(buf)
}

// docker runs a docker command and fails the test on error.
func docker(t *testing.T, args ...string) {
	t.Helper()
	out, err := exec.Command("docker", args...).CombinedOutput()
	require.NoError(t, err, string(out))
}

// approveSettings records the guarded settings in the .agenticrc.toml at rcPath as approved, as the settings prompt would.
func approveSettings(rcPath string) error {
	layers, err := config.FindLayers(filepath.Dir(rcPath))
	if err != nil {
		return err
	}
	cfg, err := config.LoadState(agenticHome)
	if err != nil {
		return err
	}

	for _, layer := range layers {
		if layer.Path == rcPath {
			return cfg.ApproveSettings(layer, agenticHome)
		}
	}
	return nil
}

// secretFile writes credentialSecret to a file outside the work dir and every tool mount, as agentic requires, and
// returns its path.
func secretFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(tempDirIn(t, rootDir), "token")
	require.NoError(t, os.WriteFile(path, []byte(credentialSecret+"\n"), 0o600))
	return path
}

// proxyLog returns every proxy access log written under the test AGENTIC_HOME, concatenated.
func proxyLog(t *testing.T) string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(agenticHome, "logs", "proxy_*.jsonl"))
	require.NoError(t, err)

	var log strings.Builder
	for _, path := range paths {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		log.Write(data)
	}
	return log.String()
}

// writableScript prints <label>=rw when path can be created (and removes it again), <label>=ro otherwise.
func writableScript(label, path string) string {
	return "touch " + path + " 2>/dev/null && rm " + path + " && echo " + label + "=rw || echo " + label + "=ro"
}
