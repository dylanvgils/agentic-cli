package docker

import "github.com/dylanvgils/agentic-cli/internal/tools"

// Package-level wrappers over a shared Client, kept until every consumer is handed a Client; remove with them.

// std is the shared Client behind the package-level wrappers.
var std = New("")

// SetContext sets the Docker context of the shared Client.
func SetContext(ctx string) { std.context = ctx }

// Context returns the Docker context of the shared Client.
func Context() string { return std.Context() }

func BuildDindImage(image string, opts tools.BuildOptions) error {
	return std.BuildDindImage(image, opts)
}

func BuildProxyImage(image, version, sourceDir string, opts tools.BuildOptions) error {
	return std.BuildProxyImage(image, version, sourceDir, opts)
}

func BuildTool(tool, image string, opts tools.BuildOptions) error {
	return std.BuildTool(tool, image, opts)
}

func BuiltTools() (map[string]bool, error) { return std.BuiltTools() }

func CheckDaemon() error { return std.CheckDaemon() }

func CleanBaseImages() error { return std.CleanBaseImages() }

func CleanImage(image string) error { return std.CleanImage(image) }

func CreateVolume(name string) error { return std.CreateVolume(name) }

func EnsureNamedVolumes(volumes []string, toolHome, containerHome, chownImage string) error {
	return std.EnsureNamedVolumes(volumes, toolHome, containerHome, chownImage)
}

func EnsureNetwork() error { return std.EnsureNetwork() }

func InspectImage(name string) (*ImageInfo, error) { return std.InspectImage(name) }

func ListAllImages(filters ...ImageFilter) ([]*ImageInfo, error) {
	return std.ListAllImages(filters...)
}

func ListContexts() ([]string, error) { return std.ListContexts() }

func ListRunningContainers() ([]*ContainerInfo, error) { return std.ListRunningContainers() }

func ListVolumeNames() ([]string, error) { return std.ListVolumeNames() }

func ListVolumes() (string, error) { return std.ListVolumes() }

func PruneBuildCache() error { return std.PruneBuildCache() }

func PruneImages() error { return std.PruneImages() }

func RemoveNetwork() error { return std.RemoveNetwork() }

func RemoveVolume(name string) error { return std.RemoveVolume(name) }

func ResolveContainerHome(image string) string { return std.ResolveContainerHome(image) }

func RunContainer(rs RunSpec, toolArgs []string) error { return std.RunContainer(rs, toolArgs) }

func SweepDindResources(toolHome string) error { return std.SweepDindResources(toolHome) }

func SweepProxyResources() error { return std.SweepProxyResources() }

func UpdateTool(tool, image string, opts tools.BuildOptions) error {
	return std.UpdateTool(tool, image, opts)
}
