package cli

import (
	"reflect"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/git"
	"github.com/dylanvgils/agentic-cli/internal/housekeeping"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/stretchr/testify/assert"
)

var _ dockerAPI = (*docker.Client)(nil)

func TestDefaultDeps(t *testing.T) {
	cases := []struct {
		name string
		want any
		got  any
	}{
		{"pruneProxyLogs", housekeeping.PruneProxyLogs, pruneProxyLogs},
		{"isTerminal", platform.IsTerminal, isTerminal},
		{"checkGitAvailable", git.CheckAvailable, checkGitAvailable},
		{"loadMarketplaceRegistry", marketplace.LoadRegistry, loadMarketplaceRegistry},
		{"saveMarketplaceRegistry", marketplace.SaveRegistry, saveMarketplaceRegistry},
	}

	for _, c := range cases {
		t.Run(c.name+" defaults to its real implementation", func(t *testing.T) {
			// Assert
			assert.Equal(t, reflect.ValueOf(c.want).Pointer(), reflect.ValueOf(c.got).Pointer())
		})
	}
}
