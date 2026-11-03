package build

import (
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
)

// fakeDocker implements Docker; a nil field succeeds with a zero value.
type fakeDocker struct {
	buildTool   func(tool, image string, opts tools.BuildOptions) error
	pruneImages func() (string, error)
}

func (f *fakeDocker) BuildTool(tool, image string, opts tools.BuildOptions) error {
	if f.buildTool == nil {
		return nil
	}
	return f.buildTool(tool, image, opts)
}

func (f *fakeDocker) PruneImages() (string, error) {
	if f.pruneImages == nil {
		return "", nil
	}
	return f.pruneImages()
}
