//go:build integration

package proxy

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/require"
)

const (
	credentialSecret = "test-secret-value"

	// proxyCredentialsDir mirrors internal/runtime/docker's dir for the proxy's CA and credential files
	proxyCredentialsDir = "/run/agentic-proxy/credentials"
)

// copyFromProxy copies file from the proxy's credentials dir, failing when it doesn't exist.
func copyFromProxy(proxy, file string) error {
	return exec.Command("docker", "cp", proxy+":"+proxyCredentialsDir+"/"+file, "-").Run()
}

// curlFromEgressNet requests url through proxy from a container on the shared egress network the proxy also
// joins, and returns curl's output with a final status=<code> line.
func curlFromEgressNet(t *testing.T, proxy, url string) string {
	t.Helper()
	ip, err := exec.Command("docker", "inspect", "--format", `{{(index .NetworkSettings.Networks "agentic-net").IPAddress}}`, proxy).Output()
	require.NoError(t, err)
	proxyURL := "http://" + strings.TrimSpace(string(ip)) + ":3128"

	out, err := exec.Command("docker", "run", "--rm", "--network", "agentic-net", "--entrypoint", "",
		platform.ToolImage, "curl", "-sS", "-m", "5", "-x", proxyURL, "-w", "\nstatus=%{http_code}", url).CombinedOutput()
	require.NoError(t, err, string(out))
	return string(out)
}

// proxyLog returns every proxy access log written under the test AGENTIC_HOME, concatenated.
func proxyLog(t *testing.T) string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(platform.AgenticHome(), "logs", "proxy_*.jsonl"))
	require.NoError(t, err)

	var log strings.Builder
	for _, path := range paths {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		log.Write(data)
	}
	return log.String()
}

// secretFile writes credentialSecret to a file outside the work dir and every tool mount, as agentic requires, and
// returns its path.
func secretFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(platform.TempDirIn(t, platform.RootDir()), "token")
	require.NoError(t, os.WriteFile(path, []byte(credentialSecret+"\n"), 0o600))
	return path
}
