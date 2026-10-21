//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pathHost has no upstream, so an allowed path gets a 502 once the proxy has decrypted the request.
const pathHost = "paths.example.test"

// TestProxyPaths checks the proxy decrypts a path rule host, which the tool trusts, and refuses paths no rule allows.
func TestProxyPaths(t *testing.T) {
	// Arrange
	dir := tempDirIn(t, rootDir)
	rcPath := filepath.Join(dir, ".agenticrc.toml")
	rc := rcContents + `
[[run.proxy.allowed_paths]]
host = "` + pathHost + `"
paths = ["/org/repo/**"]
`
	require.NoError(t, os.WriteFile(rcPath, []byte(rc), 0o644))
	require.NoError(t, approveSettings(rcPath))

	// Act
	out := runInToolIn(t, dir, pathsScript(), "--proxy")

	// Assert
	assert.Contains(t, out, "allowed=502")
	assert.Contains(t, out, "refused=403")
	assert.Contains(t, out, "proxy blocked 1 request(s) to paths no rule allows: "+pathHost+"/other/README.md")
	assert.Contains(t, proxyLog(t), `"host":"`+pathHost+`","port":"443","decision":"deny","enforced":true,"intercepted":true,"path":"/other/README.md","reason":"path-blocked"`)
}

// TestProxyCredentialPaths checks path rules limit a credential host, whose allowed paths still get the secret.
func TestProxyCredentialPaths(t *testing.T) {
	// Arrange
	secretPath := filepath.Join(tempDirIn(t, rootDir), "token")
	require.NoError(t, os.WriteFile(secretPath, []byte(credentialSecret+"\n"), 0o600))

	dir := tempDirIn(t, rootDir)
	rcPath := filepath.Join(dir, ".agenticrc.toml")
	rc := rcContents + `
[[run.proxy.credentials]]
hosts = ["` + pathHost + `"]
header = "X-Api-Key"
env = ["EXAMPLE_API_KEY"]
secret = "` + secretPath + `"

[[run.proxy.allowed_paths]]
host = "` + pathHost + `"
paths = ["/org/repo/**"]
`
	require.NoError(t, os.WriteFile(rcPath, []byte(rc), 0o644))
	require.NoError(t, approveSettings(rcPath))

	// Act
	out := runInToolIn(t, dir, pathsScript())

	// Assert
	assert.Contains(t, out, "allowed=502")
	assert.Contains(t, out, "refused=403")
	assert.NotContains(t, out, "without the placeholder")
	assert.Contains(t, proxyLog(t), `"host":"`+pathHost+`","port":"443","decision":"deny","enforced":true,"intercepted":true,"path":"/other/README.md","reason":"path-blocked"`)
}

// pathsScript runs the entrypoint, which -- skips and which adds the CA, with a claude shim requesting an allowed path
// with the credential placeholder and a refused path on pathHost.
func pathsScript() string {
	return `mkdir -p /tmp/shim
cat > /tmp/shim/claude <<'EOF'
#!/bin/sh
curl -sS -o /dev/null -w 'allowed=%{http_code}\n' -H "X-Api-Key: $EXAMPLE_API_KEY" https://` + pathHost + `/org/repo/README.md
curl -sS -o /dev/null -w 'refused=%{http_code}\n' https://` + pathHost + `/other/README.md
EOF
chmod +x /tmp/shim/claude
PATH=/tmp/shim:$PATH exec /usr/local/bin/entrypoint.sh`
}
