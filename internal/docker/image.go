package docker

import (
	"fmt"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// ImageInfo holds metadata about a built tool image.
type ImageInfo struct {
	Image              string
	Namespace          string // image namespace (e.g. "agentic", "myproject")
	Tool               string // tool name (e.g. "claude", "copilot")
	ID                 string // 12-char short ID
	Version            string // agentic.tool.version label
	Base               string // agentic.base label
	VersionArgs        string // agentic.version-args label (layer name@version pairs used to build, e.g. "node@24,java@17")
	Apt                string // agentic.apt label (comma-separated apt packages)
	CustomInstalls     string // agentic.custom-installs label (comma-separated custom install names)
	CustomInstallsHash string // agentic.custom-installs.hash label (key of the stored custom installs)
	Built              string // agentic.built label
	Pulled             string // agentic.pulled label
	CLIVersion         string // agentic.version label (CLI version that built this image)
	CacheBust          string // agentic.cachebust label (CACHEBUST build-arg baked into the tool stage)
	ProxyTrust         bool   // agentic.proxy-trust label (entrypoint trusts the proxy CA)
	Size               string // formatted size from docker image ls
}

// BuiltBefore reports whether the agentic.built label predates cutoff; a missing or invalid label counts as before.
func (i ImageInfo) BuiltBefore(cutoff time.Time) bool {
	built, ok := parseLabelTime(i.Built)
	return !ok || built.Before(cutoff)
}

// InspectImage returns metadata for the given Docker image, or nil, nil if it does not exist.
func (c *Client) InspectImage(name string) (*ImageInfo, error) {
	result, err := c.inspectImage(name)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}

	namespace, tool := resolveToolName(name, result.Config.Labels[LabelTool], result.Config.Labels[LabelNamespace])

	return &ImageInfo{
		Image:              name,
		Namespace:          namespace,
		Tool:               tool,
		ID:                 extractShortID(result.ID),
		Version:            result.Config.Labels[LabelToolVersion],
		Base:               result.Config.Labels[LabelBase],
		VersionArgs:        result.Config.Labels[LabelVersionArgs],
		Apt:                result.Config.Labels[LabelApt],
		CustomInstalls:     result.Config.Labels[LabelCustomInstalls],
		CustomInstallsHash: result.Config.Labels[LabelCustomInstallsHash],
		Built:              result.Config.Labels[LabelBuilt],
		Pulled:             result.Config.Labels[LabelPulled],
		CLIVersion:         result.Config.Labels[LabelCLIVersion],
		CacheBust:          result.Config.Labels[LabelCacheBust],
		ProxyTrust:         result.Config.Labels[LabelProxyTrust] == "true",
		Size:               c.imageSize(name),
	}, nil
}

// ListAllImages returns metadata for every agentic-managed image; optional filters (e.g. ToolFilter) narrow the result set.
func (c *Client) ListAllImages(filters ...ImageFilter) ([]*ImageInfo, error) {
	repos, err := c.listAllRepositories(filters...)
	if err != nil {
		return nil, err
	}

	var images []*ImageInfo
	for _, repo := range repos {
		info, err := c.InspectImage(repo)
		if err != nil {
			return nil, err
		}
		if info == nil {
			continue
		}
		images = append(images, info)
	}

	return images, nil
}

// ImageRefreshReason says why an agentic-built image (e.g. a sidecar) needs a rebuild, or "" if it is current; maxAge 0 skips the age check.
func ImageRefreshReason(info *ImageInfo, maxAge time.Duration) string {
	if info == nil {
		return "image missing"
	}

	if info.CLIVersion != buildinfo.Version {
		return "built by a different agentic version"
	}

	if maxAge > 0 && info.BuiltBefore(time.Now().Add(-maxAge)) {
		return fmt.Sprintf("older than %d days", int(maxAge.Hours()/24))
	}

	return ""
}

// BuiltTools returns the set of tool names that have at least one image in images.
func BuiltTools(images []*ImageInfo) map[string]bool {
	built := make(map[string]bool)
	for _, img := range images {
		built[img.Tool] = true
	}
	return built
}

// parseImageName splits an image name into namespace and tool by matching its suffix against
// the known tool names, e.g. "myproject-claude" -> ("myproject", "claude", true).
func parseImageName(image string) (namespace, tool string, ok bool) {
	for _, tool := range tools.Names() {
		suffix := "-" + tool
		if before, ok0 := strings.CutSuffix(image, suffix); ok0 {
			return before, tool, true
		}
	}
	return "", "", false
}

// resolveToolName determines the tool name and namespace for an image, preferring labels over parsing the image name.
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

// extractShortID returns the 12-character short ID from a full Docker image ID (e.g. "sha256:a1b2c3d4e5f6...").
func extractShortID(id string) string {
	if len(id) < 19 {
		return ""
	}
	return id[7:19]
}

// imageSize returns the formatted size of a Docker image, or empty string if unavailable.
func (c *Client) imageSize(name string) string {
	out, err := c.run("image", "ls", arg("format", "{{.Size}}"), referenceFilter(name))
	size := strings.TrimSpace(out)
	if err != nil || size == "" {
		return ""
	}
	return size
}

// listAllRepositories returns the repository names of every agentic-managed image, narrowed by any filters.
func (c *Client) listAllRepositories(filters ...ImageFilter) ([]string, error) {
	args := []string{
		"images",
		arg("format", "{{.Repository}}"),
		labelFilter(LabelProject, LabelProjectVal),
	}
	for _, f := range filters {
		args = append(args, string(f))
	}

	out, err := c.run(args...)
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
