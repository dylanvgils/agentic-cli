package docker

import (
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// ImageInfo holds metadata about a built tool image.
type ImageInfo struct {
	Image     string
	Namespace string // image namespace (e.g. "agentic", "myproject")
	Tool      string // tool name (e.g. "claude", "copilot")
	ID        string // 12-char short ID
	Version   string // agentic.tool.version label
	Base      string // agentic.base label
	Apt       string // agentic.apt label (comma-separated apt packages)
	Built     string // agentic.built label
	Size      string // formatted size from docker image ls
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

	namespace, tool := resolveToolName(name, result.Config.Labels[LabelTool], result.Config.Labels[LabelNamespace])

	return &ImageInfo{
		Image:     name,
		Namespace: namespace,
		Tool:      tool,
		ID:        extractShortID(result.ID),
		Version:   result.Config.Labels[LabelToolVersion],
		Base:      result.Config.Labels[LabelBase],
		Apt:       result.Config.Labels[LabelApt],
		Built:     result.Config.Labels[LabelBuilt],
		Size:      imageSize(name),
	}, nil
}

// ListAllImages returns metadata for every Docker image carrying the
// project=agentic-cli label, across all namespaces. Optional filters narrow the
// result set; use the typed constructors (e.g. ToolFilter) to build them.
func ListAllImages(filters ...ImageFilter) ([]*ImageInfo, error) {
	repos, err := listAllRepositories(filters...)
	if err != nil {
		return nil, err
	}

	var images []*ImageInfo
	for _, repo := range repos {
		info, err := InspectImage(repo)
		if err != nil || info == nil {
			continue
		}
		images = append(images, info)
	}

	return images, nil
}

// BuiltTools returns the set of tool names that have at least one built image.
func BuiltTools() (map[string]bool, error) {
	images, err := ListAllImages()
	if err != nil {
		return nil, err
	}
	return builtToolsFromImages(images), nil
}

func builtToolsFromImages(images []*ImageInfo) map[string]bool {
	built := make(map[string]bool)
	for _, img := range images {
		built[img.Tool] = true
	}
	return built
}

// parseImageName splits an image name into namespace and tool by matching the
// suffix against the known set of tool names.
// e.g. "myproject-claude" → ("myproject", "claude", true)
func parseImageName(image string) (namespace, tool string, ok bool) {
	for _, tool := range tools.Names() {
		suffix := "-" + tool
		if before, ok0 := strings.CutSuffix(image, suffix); ok0 {
			return before, tool, true
		}
	}
	return "", "", false
}

// resolveToolName determines the tool name and namespace for an image.
// Label values take precedence; falls back to parsing the image name.
func resolveToolName(image, labelTool, labelNamespace string) (namespace, tool string) {
	parsedNamespace, parsedTool, _ := parseImageName(image)
	tool = labelTool
	if tool == "" {
		tool = parsedTool
	}

	namespace = labelNamespace
	if namespace == "" {
		namespace = parsedNamespace
	}
	return
}

// extractShortID returns the 12-character short ID from a full Docker image ID
// (e.g. "sha256:a1b2c3d4e5f6..."). Returns empty string if the ID is too short.
func extractShortID(id string) string {
	if len(id) < 19 {
		return ""
	}
	return id[7:19]
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

// listAllRepositories returns the repository names of every Docker image
// carrying the project=agentic-cli label. Optional extraFilters are passed
// as additional --filter flags.
func listAllRepositories(filters ...ImageFilter) ([]string, error) {
	args := []string{
		"images",
		arg("format", "{{.Repository}}"),
		labelFilter(LabelProject, LabelProjectVal),
	}
	for _, f := range filters {
		args = append(args, string(f))
	}

	out, err := dockerRun(args...)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var repos []string
	for repo := range strings.FieldsSeq(out) {
		if repo == "<none>" || seen[repo] {
			continue
		}
		seen[repo] = true
		repos = append(repos, repo)
	}

	return repos, nil
}
