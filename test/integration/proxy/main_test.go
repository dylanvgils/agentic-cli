//go:build integration

// Package proxy checks what the agentic-proxy image does on its own, against a real upstream: startup, forwarding,
// the address guard, SNI, credential injection, path rules and client auth.
package proxy

import (
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
)

func TestMain(m *testing.M) {
	os.Exit(platform.ProxyMain(m))
}
