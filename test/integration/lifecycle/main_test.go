//go:build integration

// Package lifecycle checks a run cleans up after itself: on a closed terminal and for orphans of crashed runs.
package lifecycle

import (
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
)

func TestMain(m *testing.M) {
	os.Exit(platform.Main(m))
}
