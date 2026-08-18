package build

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/tools"
)

func stubBuildTool(t *testing.T, fn func(tool, image string, opts tools.BuildOptions) error) {
	t.Helper()
	orig := BuildTool
	BuildTool = fn
	t.Cleanup(func() { BuildTool = orig })
}
