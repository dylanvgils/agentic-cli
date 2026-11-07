//go:build integration

// Package egress checks how agentic routes a tool's egress through agentic-proxy, on every runtime: allowlist, monitor
// mode, credentials, path rules, client access and integrations. What the proxy does on its own is in the proxy package.
package egress

import (
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
)

func TestMain(m *testing.M) {
	os.Exit(platform.RuntimesMain(m))
}
