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

// runInTool runs script with sh inside the test tool container, passing flags to agentic run, and returns the combined output.
func runInTool(t *testing.T, script string, flags ...string) string {
	t.Helper()
	return runInToolIn(t, workDir, script, flags...)
}

// runInToolIn is runInTool with dir as the cwd, so its .agenticrc.toml applies.
func runInToolIn(t *testing.T, dir, script string, flags ...string) string {
	t.Helper()
	args := append([]string{"run", "--trust-dir"}, flags...)
	args = append(args, testTool, "--", "sh", "-c", script)

	out, err := agenticCmdIn(dir, args...).CombinedOutput()
	t.Logf("agentic %s\n%s", strings.Join(args, " "), out)
	require.NoError(t, err)
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

// approveCredentials records the credentials in the .agenticrc.toml at rcPath as approved, as the trust prompt would.
func approveCredentials(t *testing.T, rcPath string) {
	t.Helper()
	layers, err := config.FindLayers(filepath.Dir(rcPath))
	require.NoError(t, err)
	cfg, err := config.LoadConfig(agenticHome)
	require.NoError(t, err)

	for _, layer := range layers {
		if creds := layer.RC.Run.Proxy.Credentials; len(creds) > 0 {
			require.NoError(t, cfg.ApproveCredentials(layer.Path, config.CredentialsHash(creds), agenticHome))
		}
	}
}

// approveSettings records the guarded settings in the .agenticrc.toml at rcPath as approved, as the settings prompt would.
func approveSettings(rcPath string) error {
	layers, err := config.FindLayers(filepath.Dir(rcPath))
	if err != nil {
		return err
	}
	cfg, err := config.LoadConfig(agenticHome)
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
