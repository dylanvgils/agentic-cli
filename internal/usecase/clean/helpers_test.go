package clean

import (
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
)

// fakeDocker implements Docker; a nil field succeeds with a zero value.
type fakeDocker struct {
	listAllImages   func(...docker.ImageFilter) ([]*docker.ImageInfo, error)
	listRunning     func() ([]*docker.ContainerInfo, error)
	cleanImage      func(string) error
	cleanBaseImages func() error
	sweepRuns       func(string) (docker.SweptRuns, error)
	removeNetwork   func() error
	pruneImages     func() (string, error)
	pruneBuildCache func() (string, error)
}

func (f *fakeDocker) ListAllImages(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
	if f.listAllImages == nil {
		return nil, nil
	}
	return f.listAllImages(filters...)
}

func (f *fakeDocker) ListRunningContainers() ([]*docker.ContainerInfo, error) {
	if f.listRunning == nil {
		return nil, nil
	}
	return f.listRunning()
}

func (f *fakeDocker) CleanImage(image string) error {
	if f.cleanImage == nil {
		return nil
	}
	return f.cleanImage(image)
}

func (f *fakeDocker) CleanBaseImages() error {
	if f.cleanBaseImages == nil {
		return nil
	}
	return f.cleanBaseImages()
}

func (f *fakeDocker) SweepRuns(toolHome string) (docker.SweptRuns, error) {
	if f.sweepRuns == nil {
		return docker.SweptRuns{}, nil
	}
	return f.sweepRuns(toolHome)
}

func (f *fakeDocker) RemoveNetwork() error {
	if f.removeNetwork == nil {
		return nil
	}
	return f.removeNetwork()
}

func (f *fakeDocker) PruneImages() (string, error) {
	if f.pruneImages == nil {
		return "", nil
	}
	return f.pruneImages()
}

func (f *fakeDocker) PruneBuildCache() (string, error) {
	if f.pruneBuildCache == nil {
		return "", nil
	}
	return f.pruneBuildCache()
}

// fakeRuntime implements ImageCleaner; a nil field succeeds.
type fakeRuntime struct {
	cleanImage func(string) error
}

func (f *fakeRuntime) CleanImage(image string) error {
	if f.cleanImage == nil {
		return nil
	}
	return f.cleanImage(image)
}

// recordCleans returns a CleanImage func that appends prefix plus each image to cleaned.
func recordCleans(cleaned *[]string, prefix string) func(string) error {
	return func(image string) error {
		*cleaned = append(*cleaned, prefix+image)
		return nil
	}
}
