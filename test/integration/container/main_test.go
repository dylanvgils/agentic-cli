//go:build integration

// Package container checks the kernel enforces the tool container's security and resource args and its mounts.
package container

import (
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
)

func TestMain(m *testing.M) {
	os.Exit(platform.Main(m))
}
