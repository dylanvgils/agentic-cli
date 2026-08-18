package run

import (
	"reflect"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/stretchr/testify/assert"
)

func TestDefaultDeps(t *testing.T) {
	t.Run("EnsureNamedVolumes defaults to docker.EnsureNamedVolumes", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(docker.EnsureNamedVolumes).Pointer(), reflect.ValueOf(EnsureNamedVolumes).Pointer())
	})

	t.Run("EnsureNetwork defaults to docker.EnsureNetwork", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(docker.EnsureNetwork).Pointer(), reflect.ValueOf(EnsureNetwork).Pointer())
	})

	t.Run("SyncMarketplaces defaults to marketplace.Sync", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(marketplace.Sync).Pointer(), reflect.ValueOf(SyncMarketplaces).Pointer())
	})

	t.Run("RecordMarketplaceUsage defaults to marketplace.RecordUsage", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(marketplace.RecordUsage).Pointer(), reflect.ValueOf(RecordMarketplaceUsage).Pointer())
	})
}
