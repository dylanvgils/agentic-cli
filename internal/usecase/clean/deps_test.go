package clean

import (
	"reflect"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/housekeeping"
	"github.com/stretchr/testify/assert"
)

func TestDefaultDeps(t *testing.T) {
	t.Run("ListAllImages defaults to docker.ListAllImages", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(docker.ListAllImages).Pointer(), reflect.ValueOf(ListAllImages).Pointer())
	})

	t.Run("CleanImage defaults to docker.CleanImage", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(docker.CleanImage).Pointer(), reflect.ValueOf(CleanImage).Pointer())
	})

	t.Run("CleanBaseImages defaults to docker.CleanBaseImages", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(docker.CleanBaseImages).Pointer(), reflect.ValueOf(CleanBaseImages).Pointer())
	})

	t.Run("SweepProxyResources defaults to docker.SweepProxyResources", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(docker.SweepProxyResources).Pointer(), reflect.ValueOf(SweepProxyResources).Pointer())
	})

	t.Run("RemoveNetwork defaults to docker.RemoveNetwork", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(docker.RemoveNetwork).Pointer(), reflect.ValueOf(RemoveNetwork).Pointer())
	})

	t.Run("pruneAuditLogs defaults to housekeeping.PruneJSONLLogs", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(housekeeping.PruneJSONLLogs).Pointer(), reflect.ValueOf(pruneAuditLogs).Pointer())
	})
}
