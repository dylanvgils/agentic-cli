//go:build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// guardHost is the fake upstream's alias on agentic-net, so it resolves to a private address.
const guardHost = "upstream.guard.test"

// TestProxyAddressGuard checks an allowed host can't reach local or private addresses unless listed exactly.
func TestProxyAddressGuard(t *testing.T) {
	fakeContainer(t).onNetwork("agentic-net").withAlias(guardHost).listeningOn("80").create()

	t.Run("wildcard entry cannot reach a private address", func(t *testing.T) {
		// Arrange
		dir := proxyWorkDir(t, ".guard.test")

		// Act
		out := runInToolIn(t, dir, connectScript(guardHost), "--proxy")

		// Assert
		assert.Contains(t, out, "connect=403")
		assert.Contains(t, proxyLog(t), `"host":"`+guardHost+`","port":"80","decision":"deny","enforced":true,"reason":"blocked-address"`)
	})

	t.Run("exact entry can reach a private address", func(t *testing.T) {
		// Arrange
		dir := proxyWorkDir(t, guardHost)

		// Act
		out := runInToolIn(t, dir, connectScript(guardHost), "--proxy")

		// Assert
		assert.Contains(t, out, "connect=200")
	})

	t.Run("exact entry cannot reach loopback", func(t *testing.T) {
		// Arrange
		dir := proxyWorkDir(t, "localhost")

		// Act
		out := runInToolIn(t, dir, connectScript("localhost"), "--proxy")

		// Assert
		assert.Contains(t, out, "connect=403")
	})
}

// connectScript prints the proxy's CONNECT status for host:80, also for hosts NO_PROXY lists; the fake upstream never answers HTTP, so curl times out after it.
func connectScript(host string) string {
	return "curl -s -o /dev/null -m 3 --noproxy '' --proxytunnel -w 'connect=%{http_connect}\\n' http://" + host + "/ || true"
}
