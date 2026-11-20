package cli

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/microsandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/watch"
	"github.com/stretchr/testify/assert"
)

var (
	_ dockerAPI = (*docker.Client)(nil)
	_ msbAPI    = (*microsandbox.Client)(nil)
)

func Test_spawnedWatchdogs_For(t *testing.T) {
	// Act
	got := spawnedWatchdogs{}.For("prod")

	// Assert
	assert.Equal(t, watch.NewSpawner(watchCmdName, "prod"), got)
}
