package docker

import (
	"encoding/json"
	"strings"
)

// ImageInfo holds metadata about a built tool image.
type ImageInfo struct {
	Image   string
	ID      string // 12-char short ID
	Version string // agentic.tool.version label
	Base    string // agentic.base label
	Built   string // agentic.built label
	SizeMB  int
}

type imageInspectResult struct {
	ID     string `json:"Id"`
	Size   int64  `json:"Size"`
	Config struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
}

// InspectImage returns metadata for the given Docker image.
// Returns nil, nil if the image does not exist or is not built.
func InspectImage(name string) (*ImageInfo, error) {
	out, err := dockerRun("inspect", "--format={{json .}}", name)
	if err != nil {
		return nil, nil
	}

	out = strings.TrimSpace(out)
	var result imageInspectResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, err
	}

	shortID := ""
	if len(result.ID) >= 19 {
		shortID = result.ID[7:19]
	}

	return &ImageInfo{
		Image:   name,
		ID:      shortID,
		Version: result.Config.Labels["agentic.tool.version"],
		Base:    result.Config.Labels["agentic.base"],
		Built:   result.Config.Labels["agentic.built"],
		SizeMB:  int(result.Size / 1048576),
	}, nil
}
