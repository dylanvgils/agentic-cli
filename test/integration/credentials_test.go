//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// credentialHost has no upstream, so the proxy answers 502 once it has decrypted the request.
	credentialHost = "upstream.example.test"

	credentialSecret = "test-secret-value"
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
	approveCredentials(t, rcPath)
	require.NoError(t, approveSettings(rcPath))

	t.Run("env holds the placeholder, not the secret", func(t *testing.T) {
		// Act
		out := runInToolIn(t, dir, `echo "key=$EXAMPLE_API_KEY"; env | grep -q '`+credentialSecret+`' && echo leak=yes || echo leak=no`)

		// Assert
		assert.Contains(t, out, "agentic: injecting credentials for "+credentialHost+" ("+secretPath+")")
		assert.Contains(t, out, "key=agentic-proxy-managed")
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
		assert.Contains(t, proxyLog(t), `"host":"`+credentialHost+`","port":"443","decision":"allow","enforced":true,"injected":true`)
	})
}
