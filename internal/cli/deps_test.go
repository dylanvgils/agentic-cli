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

func TestDefaultDeps(t *testing.T) {
	cases := []struct {
		name string
		want any
		got  any
	}{
		{"checkDockerDaemon", docker.CheckDaemon, checkDockerDaemon},
		{"buildProxyImage", docker.BuildProxyImage, buildProxyImage},
		{"runContainer", docker.RunContainer, runContainer},
		{"inspectImage", docker.InspectImage, inspectImage},
		{"builtTools", docker.BuiltTools, builtTools},
		{"listAllImages", docker.ListAllImages, listAllImages},
		{"cleanImage", docker.CleanImage, cleanImage},
		{"pruneImages", docker.PruneImages, pruneImages},
		{"pruneBuildCache", docker.PruneBuildCache, pruneBuildCache},
		{"pruneProxyLogs", housekeeping.PruneJSONLLogs, pruneProxyLogs},
		{"createVolume", docker.CreateVolume, createVolume},
		{"listVolumes", docker.ListVolumes, listVolumes},
		{"listVolumeNames", docker.ListVolumeNames, listVolumeNames},
		{"removeVolume", docker.RemoveVolume, removeVolume},
		{"listRunningContainers", docker.ListRunningContainers, listRunningContainers},
		{"isTerminal", platform.IsTerminal, isTerminal},
		{"setContext", docker.SetContext, setContext},
		{"listContexts", docker.ListContexts, listContexts},
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
