//go:build integration

package proxy

import (
	"os"
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

// connectScript prints the proxy's CONNECT status for host:80, also for hosts NO_PROXY lists; the fake upstream never answers HTTP, so curl times out after it.
func connectScript(host string) string {
	return "curl -s -o /dev/null -m 3 --noproxy '' --proxytunnel -w 'connect=%{http_connect}\\n' http://" + host + "/ || true"
}

// copyFromProxy reports whether file could be copied from the proxy's credentials dir, false when it doesn't exist.
func copyFromProxy(proxy, file string) bool {
	return platform.DockerOK("cp", proxy+":"+proxyCredentialsDir+"/"+file, "-")
}

// curlFromEgressNet requests url through proxy from a container on the shared egress network the proxy also
// joins, and returns curl's output with a final status=<code> line.
func curlFromEgressNet(t *testing.T, proxy, url string) string {
	t.Helper()
	ip := platform.Docker(t, "inspect", "--format", `{{(index .NetworkSettings.Networks "agentic-net").IPAddress}}`, proxy)
	proxyURL := "http://" + ip + ":3128"

	return platform.Docker(t, "run", "--rm", "--network", "agentic-net", "--entrypoint", "",
		platform.ToolImage, "curl", "-sS", "-m", "5", "-x", proxyURL, "-w", "\nstatus=%{http_code}", url)
}

// pathsScript requests an allowed path with the credential placeholder and a refused path on pathHost.
func pathsScript() string {
	return `curl -sS -o /dev/null -w 'allowed=%{http_code}\n' -H "X-Api-Key: $EXAMPLE_API_KEY" https://` + pathHost + `/org/repo/README.md
curl -sS -o /dev/null -w 'refused=%{http_code}\n' https://` + pathHost + `/other/README.md`
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
