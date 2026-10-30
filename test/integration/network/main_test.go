//go:build integration

// Package network checks the run networks isolate the tool container, with and without --proxy.
package network

import (
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
)

func TestMain(m *testing.M) {
	os.Exit(platform.Main(m))
}
