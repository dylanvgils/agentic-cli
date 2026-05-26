package tools

import (
	"testing"

	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
	"github.com/stretchr/testify/assert"
)

// --- AptInstallRun ---
func TestAptInstallRun_rendersUpdateInstallCleanup(t *testing.T) {
	// Arrange
	pkgs := []string{"curl", "wget"}

	// Act
	run := AptInstallRun(pkgs)
	result := df.File{Stages: []df.Stage{
		df.NewStage(df.From{Image: "scratch"}).Add(run).Build(),
	}}.Render()

	// Assert
	assert.Contains(t, result, "apt-get update -yq")
	assert.Contains(t, result, "apt-get install -yq --no-install-recommends")
	assert.Contains(t, result, "curl")
	assert.Contains(t, result, "wget")
	assert.Contains(t, result, "rm -rf /var/lib/apt/lists/*")
}
