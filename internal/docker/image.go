package docker

import (
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// ImageInfo holds metadata about a built tool image.
type ImageInfo struct {
	Image   string
	Prefix  string // image name prefix (e.g. "agentic", "myproject")
	Tool    string // tool name (e.g. "claude", "copilot")
	ID      string // 12-char short ID
	Version string // agentic.tool.version label
	Base    string // agentic.base label
	Apt     string // agentic.apt label (comma-separated apt packages)
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

	labelTool := result.Config.Labels[LabelTool]
	prefix, parsedTool, _ := parseImageName(name)
	tool := labelTool
	if tool == "" {
		tool = parsedTool
	}

	return &ImageInfo{
		Image:   name,
		Prefix:  prefix,
		Tool:    tool,
		ID:      shortID,
		Version: result.Config.Labels[LabelToolVersion],
		Base:    result.Config.Labels[LabelBase],
		Apt:     result.Config.Labels[LabelApt],
		Built:   result.Config.Labels[LabelBuilt],
		Size:    imageSize(name),
	}, nil
}

// ListAllAgenticImages returns metadata for every Docker image carrying the
// project=agentic-cli label, across all prefixes.
func ListAllAgenticImages() ([]*ImageInfo, error) {
	out, err := dockerRun("images",
		arg("format", "{{.Repository}}"),
		labelFilter(LabelProject, LabelProjectVal),
	)
	if err != nil {
		return nil, err
	}

	var images []*ImageInfo
	for name := range strings.FieldsSeq(out) {
		if name == "<none>" {
			continue
		}
		info, err := InspectImage(name)
		if err != nil || info == nil {
			continue
		}
		images = append(images, info)
	}

	return images, nil
}

// parseImageName splits an image name into prefix and tool by matching the
// suffix against the known set of tool names.
// e.g. "myproject-claude" → ("myproject", "claude", true)
func parseImageName(image string) (prefix, tool string, ok bool) {
	for _, t := range tools.Names() {
		suffix := "-" + t
		if strings.HasSuffix(image, suffix) {
			return strings.TrimSuffix(image, suffix), t, true
		}
	}
	return "", "", false
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
