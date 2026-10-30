//go:build integration

// Package microsandbox checks tools run in a microsandbox VM with their mounts, limits and egress allowlist applied.
package microsandbox

import (
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
)

func TestMain(m *testing.M) {
	os.Exit(platform.MicrosandboxMain(m))
}
