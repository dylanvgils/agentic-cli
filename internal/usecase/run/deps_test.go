package run

import (
	"reflect"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/msb"
	"github.com/stretchr/testify/assert"
)

var (
	_ Docker = (*docker.Client)(nil)
	_ Msb    = (*msb.Client)(nil)
)

func TestDefaultDeps(t *testing.T) {
	t.Run("SyncMarketplaces defaults to marketplace.Sync", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(marketplace.Sync).Pointer(), reflect.ValueOf(SyncMarketplaces).Pointer())
	})

	t.Run("RecordMarketplaceUsage defaults to marketplace.RecordUsage", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(marketplace.RecordUsage).Pointer(), reflect.ValueOf(RecordMarketplaceUsage).Pointer())
	})

	t.Run("MsbClient defaults to an msb.Client", func(t *testing.T) {
		// Assert
		assert.IsType(t, &msb.Client{}, MsbClient)
	})
}
