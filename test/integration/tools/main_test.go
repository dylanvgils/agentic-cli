//go:build integration

// Package tools smoke-tests every tool's image.
package tools

import (
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
)

func TestMain(m *testing.M) {
	os.Exit(platform.Main(m))
}
