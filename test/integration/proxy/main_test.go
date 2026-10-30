//go:build integration

// Package proxy checks the egress proxy: address guard, clients, credentials, path rules and integrations.
package proxy

import (
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
)

func TestMain(m *testing.M) {
	os.Exit(platform.Main(m))
}
