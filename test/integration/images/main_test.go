//go:build integration

// Package images smoke-tests every tool's image and every extra base layer.
package images

import (
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
)

func TestMain(m *testing.M) {
	os.Exit(platform.SmokeMain(m))
}
