//go:build integration

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// credentialHost has no upstream, so the proxy answers 502 once it has decrypted the request.
	credentialHost = "upstream.example.test"

	credentialSecret = "test-secret-value"

	// proxyCredentialsDir mirrors internal/docker's dir for the proxy's CA and credential files
	proxyCredentialsDir = "/run/agentic-proxy/credentials"
)

// TestProxyCredentials checks the tool sees only the placeholder and trusts the proxy CA for credential hosts.
func TestProxyCredentials(t *testing.T) {
	// The secret sits outside the work dir and every tool mount, which agentic requires
	secretPath := filepath.Join(tempDirIn(t, rootDir), "token")
	require.NoError(t, os.WriteFile(secretPath, []byte(credentialSecret+"\n"), 0o600))

	dir := tempDirIn(t, rootDir)
	rcPath := filepath.Join(dir, ".agenticrc.toml")
	rc := rcContents + `
[[run.proxy.credentials]]
hosts = ["` + credentialHost + `"]
header = "X-Api-Key"
env = ["EXAMPLE_API_KEY"]
secret = "` + secretPath + `"
`
	require.NoError(t, os.WriteFile(rcPath, []byte(rc), 0o644))
	require.NoError(t, approveSettings(rcPath))

	t.Run("env holds the placeholder, not the secret", func(t *testing.T) {
		// Act
		out := runInToolIn(t, dir, `echo "key=$EXAMPLE_API_KEY"; env | grep -q '`+credentialSecret+`' && echo leak=yes || echo leak=no`)

		// Assert
		assert.Contains(t, out, "agentic: injecting credentials for "+credentialHost+" ("+secretPath+")")
		assert.Regexp(t, `key=agentic-[0-9a-f]{24}\n`, out)
		assert.Contains(t, out, "leak=no")
	})

	t.Run("tool trusts the proxy CA for credential hosts", func(t *testing.T) {
		// Arrange
		// -- skips the entrypoint that adds the CA, so run it with a claude shim that does the request
		script := `mkdir -p /tmp/shim
cat > /tmp/shim/claude <<'EOF'
#!/bin/sh
curl -sS -o /dev/null -w 'status=%{http_code}\n' https://` + credentialHost + `/ && echo tls=ok
EOF
chmod +x /tmp/shim/claude
PATH=/tmp/shim:$PATH exec /usr/local/bin/entrypoint.sh`

		// Act
		out := runInToolIn(t, dir, script)

		// Assert
		assert.Contains(t, out, "status=502")
		assert.Contains(t, out, "tls=ok")
		assert.NotContains(t, out, "without the placeholder")
		assert.Contains(t, proxyLog(t), `"host":"`+credentialHost+`","port":"443","decision":"allow","enforced":true,"intercepted":true`)
	})

	t.Run("proxy deletes the CA key and credentials once loaded", func(t *testing.T) {
		// Arrange
		copyOut := func(proxy, file string) error {
			return exec.Command("docker", "cp", proxy+":"+proxyCredentialsDir+"/"+file, "-").Run()
		}
		var keyErr, credsErr, certErr error

		// Act
		duringRun(t, dir, func() {
			proxy := runningProxy(t)
			keyErr = copyOut(proxy, "ca-key.pem")
			credsErr = copyOut(proxy, "credentials.json")
			certErr = copyOut(proxy, "ca.pem")
		})

		// Assert - the CA cert stays, which proves the path is right
		assert.Error(t, keyErr)
		assert.Error(t, credsErr)
		assert.NoError(t, certErr)
	})

	t.Run("proxy refuses clients outside the run's network", func(t *testing.T) {
		// Arrange
		var out []byte

		// Act - a container on the shared egress network the proxy also joins
		duringRun(t, dir, func() {
			proxy := runningProxy(t)
			ip, err := exec.Command("docker", "inspect", "--format", `{{(index .NetworkSettings.Networks "agentic-net").IPAddress}}`, proxy).Output()
			require.NoError(t, err)
			proxyURL := "http://" + strings.TrimSpace(string(ip)) + ":3128"

			out, err = exec.Command("docker", "run", "--rm", "--network", "agentic-net", "--entrypoint", "",
				testNamespace+"-"+testTool, "curl", "-sS", "-m", "5", "-x", proxyURL, "-w", "\nstatus=%{http_code}", "http://"+credentialHost+"/").CombinedOutput()
			require.NoError(t, err, string(out))
		})

		// Assert
		assert.Contains(t, string(out), "client not on the agentic proxy network")
		assert.Contains(t, string(out), "status=403")
	})
}
