package docker

import "strings"

// ImageInfo holds metadata about a built tool image.
type ImageInfo struct {
	Image   string
	ID      string // 12-char short ID
	Version string // agentic.tool.version label
	Base    string // agentic.base label
	Built   string // agentic.built label
	Size    string // formatted size from docker image ls
}

// InspectImage returns metadata for the given Docker image.
// Returns nil, nil if the image does not exist or is not built.
func InspectImage(name string) (*ImageInfo, error) {
	result, err := inspectImage(name)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}

	shortID := ""
	if len(result.ID) >= 19 {
		shortID = result.ID[7:19]
	}

	return &ImageInfo{
		Image:   name,
		ID:      shortID,
		Version: result.Config.Labels[LabelToolVersion],
		Base:    result.Config.Labels[LabelBase],
		Built:   result.Config.Labels[LabelBuilt],
		Size:    imageSize(name),
	}, nil
}

// imageSize returns the formatted size of a Docker image, or empty string if unavailable.
func imageSize(name string) string {
	out, err := dockerRun("image", "ls", arg("format", "{{.Size}}"), referenceFilter(name))
	size := strings.TrimSpace(out)
	if err != nil || size == "" {
		return ""
	}
	return size
}
