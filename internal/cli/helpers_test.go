package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/build"
	"github.com/dylanvgils/agentic-cli/internal/usecase/clean"
	"github.com/dylanvgils/agentic-cli/internal/usecase/run"
	"github.com/dylanvgils/agentic-cli/internal/usecase/toolupdate"
	"github.com/dylanvgils/agentic-cli/internal/usecase/update"
	"github.com/stretchr/testify/require"
)

// captureStdout replaces os.Stdout with a pipe and returns what was written; for logging.Step/Detail-based output, use captureLog instead.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stdout
	os.Stdout = w
	fn()
	w.Close() //nolint:errcheck
	os.Stdout = orig
	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck
	return buf.String()
}

// captureLog swaps logging.Log for the duration of fn and returns what was written.
func captureLog(t *testing.T, fn func()) string {
	t.Helper()

	var buf bytes.Buffer
	orig := logging.Log
	logging.Log = logging.New(&buf)
	t.Cleanup(func() { logging.Log = orig })

	fn()

	return buf.String()
}

// captureRunContainer stubs runContainer, run's ensure-volumes/network calls, and inspectImage, returning a getter for the captured RunSpec and tool args.
func captureRunContainer(t *testing.T) func() (docker.RunSpec, []string) {
	t.Helper()
	var capturedSpec docker.RunSpec
	var capturedArgs []string

	origRun := runContainer
	runContainer = func(rs docker.RunSpec, args []string) error {
		capturedSpec = rs
		capturedArgs = args
		return nil
	}

	origEnsure := run.EnsureNamedVolumes
	run.EnsureNamedVolumes = func(volumes []string, toolHome, containerHome, chownImage string) error {
		return nil
	}

	origEnsureNet := run.EnsureNetwork
	run.EnsureNetwork = func() error { return nil }

	fakeInspect := func(name string) (*docker.ImageInfo, error) {
		return &docker.ImageInfo{Image: name}, nil
	}

	origInspect := inspectImage
	inspectImage = fakeInspect

	origToolUpdateInspect := toolupdate.InspectImage
	toolupdate.InspectImage = fakeInspect

	t.Cleanup(func() {
		runContainer = origRun
		run.EnsureNamedVolumes = origEnsure
		run.EnsureNetwork = origEnsureNet
		inspectImage = origInspect
		toolupdate.InspectImage = origToolUpdateInspect
	})

	return func() (docker.RunSpec, []string) { return capturedSpec, capturedArgs }
}

// findVolumeByContainerPath returns the one volume spec ending in containerPath, failing the test if there isn't exactly one match.
func findVolumeByContainerPath(t *testing.T, volumes []string, containerPath string) string {
	t.Helper()
	var matches []string
	for _, v := range volumes {
		if strings.HasSuffix(v, ":"+containerPath) {
			matches = append(matches, v)
		}
	}
	require.Len(t, matches, 1, "expected exactly one volume mounted at %s, got %v", containerPath, volumes)
	return matches[0]
}

// withTempToolHome sets toolHome to a temp dir and pre-trusts the dirs tests run in (os.TempDir() for t.Chdir, cwd otherwise).
func withTempToolHome(t *testing.T) {
	t.Helper()
	homeDir := t.TempDir()
	cwd, err := os.Getwd()
	require.NoError(t, err)
	cfg := &config.CliConfig{TrustedDirs: []string{os.TempDir(), cwd}}
	require.NoError(t, cfg.Save(homeDir))
	orig := toolHome
	toolHome = homeDir
	t.Cleanup(func() { toolHome = orig })
}

// writeTrustConfig saves a CliConfig with the given trusted dirs into toolHome.
func writeTrustConfig(t *testing.T, toolHome string, dirs []string) {
	t.Helper()
	cfg := &config.CliConfig{TrustedDirs: dirs}
	require.NoError(t, cfg.Save(toolHome))
}

func stubBuiltTools(t *testing.T, fn func() (map[string]bool, error)) {
	t.Helper()
	orig := builtTools
	builtTools = fn
	t.Cleanup(func() { builtTools = orig })
}

func stubBuildTool(t *testing.T, fn func(tool, image string, opts tools.BuildOptions) error) {
	t.Helper()
	orig := build.BuildTool
	build.BuildTool = fn
	t.Cleanup(func() { build.BuildTool = orig })
}

func stubBuildProxyImage(t *testing.T, fn func(image, version, sourceDir string, opts tools.BuildOptions) error) {
	t.Helper()
	orig := buildProxyImage
	buildProxyImage = fn
	t.Cleanup(func() { buildProxyImage = orig })
}

func stubCheckDockerDaemon(t *testing.T, fn func() error) {
	t.Helper()
	orig := checkDockerDaemon
	checkDockerDaemon = fn
	t.Cleanup(func() { checkDockerDaemon = orig })
}

func stubCleanImage(t *testing.T, fn func(string) error) {
	t.Helper()
	orig := cleanImage
	cleanImage = fn
	t.Cleanup(func() { cleanImage = orig })
}

func stubCleanCleanImage(t *testing.T, fn func(string) error) {
	t.Helper()
	orig := clean.CleanImage
	clean.CleanImage = fn
	t.Cleanup(func() { clean.CleanImage = orig })
}

func stubCleanCleanBaseImages(t *testing.T, fn func() error) {
	t.Helper()
	orig := clean.CleanBaseImages
	clean.CleanBaseImages = fn
	t.Cleanup(func() { clean.CleanBaseImages = orig })
}

func stubCleanListAllImages(t *testing.T, fn func(...docker.ImageFilter) ([]*docker.ImageInfo, error)) {
	t.Helper()
	orig := clean.ListAllImages
	clean.ListAllImages = fn
	t.Cleanup(func() { clean.ListAllImages = orig })
}

func stubCleanSweepProxyResources(t *testing.T, fn func() error) {
	t.Helper()
	orig := clean.SweepProxyResources
	clean.SweepProxyResources = fn
	t.Cleanup(func() { clean.SweepProxyResources = orig })
}

func stubCleanRemoveNetwork(t *testing.T, fn func() error) {
	t.Helper()
	orig := clean.RemoveNetwork
	clean.RemoveNetwork = fn
	t.Cleanup(func() { clean.RemoveNetwork = orig })
}

func stubCreateVolume(t *testing.T, fn func(string) error) {
	t.Helper()
	orig := createVolume
	createVolume = fn
	t.Cleanup(func() { createVolume = orig })
}

func stubInspectImage(t *testing.T, info *docker.ImageInfo, err error) {
	t.Helper()
	orig := inspectImage
	inspectImage = func(_ string) (*docker.ImageInfo, error) { return info, err }
	t.Cleanup(func() { inspectImage = orig })
}

func stubListAllImages(t *testing.T, fn func(...docker.ImageFilter) ([]*docker.ImageInfo, error)) {
	t.Helper()
	orig := listAllImages
	listAllImages = fn
	t.Cleanup(func() { listAllImages = orig })
}

func stubListRunningContainers(t *testing.T, fn func() ([]*docker.ContainerInfo, error)) {
	t.Helper()
	orig := listRunningContainers
	listRunningContainers = fn
	t.Cleanup(func() { listRunningContainers = orig })
}

func stubListVolumeNames(t *testing.T, fn func() ([]string, error)) {
	t.Helper()
	orig := listVolumeNames
	listVolumeNames = fn
	t.Cleanup(func() { listVolumeNames = orig })
}

func stubListVolumes(t *testing.T, fn func() (string, error)) {
	t.Helper()
	orig := listVolumes
	listVolumes = fn
	t.Cleanup(func() { listVolumes = orig })
}

func stubSetContext(t *testing.T, fn func(string)) {
	t.Helper()
	orig := setContext
	setContext = fn
	t.Cleanup(func() { setContext = orig })
}

func stubListContexts(t *testing.T, fn func() ([]string, error)) {
	t.Helper()
	orig := listContexts
	listContexts = fn
	t.Cleanup(func() { listContexts = orig })
}

func stubPruneImages(t *testing.T, fn func() error) {
	t.Helper()
	orig := pruneImages
	pruneImages = fn
	t.Cleanup(func() { pruneImages = orig })
}

func stubPruneBuildCache(t *testing.T, fn func() error) {
	t.Helper()
	orig := pruneBuildCache
	pruneBuildCache = fn
	t.Cleanup(func() { pruneBuildCache = orig })
}

func stubRemoveVolume(t *testing.T, fn func(string) error) {
	t.Helper()
	orig := removeVolume
	removeVolume = fn
	t.Cleanup(func() { removeVolume = orig })
}

func stubPruneProxyLogs(t *testing.T, fn func(dir string, maxAge time.Duration)) {
	t.Helper()
	orig := pruneProxyLogs
	pruneProxyLogs = fn
	t.Cleanup(func() { pruneProxyLogs = orig })
}

func stubUpdateInspectImage(t *testing.T, info *docker.ImageInfo, err error) {
	t.Helper()
	orig := update.InspectImage
	update.InspectImage = func(_ string) (*docker.ImageInfo, error) { return info, err }
	t.Cleanup(func() { update.InspectImage = orig })
}

func stubUpdateListAllImages(t *testing.T, fn func(...docker.ImageFilter) ([]*docker.ImageInfo, error)) {
	t.Helper()
	orig := update.ListAllImages
	update.ListAllImages = fn
	t.Cleanup(func() { update.ListAllImages = orig })
}

func stubUpdateUpdateTool(t *testing.T, fn func(tool, image string, opts tools.BuildOptions) error) {
	t.Helper()
	orig := update.UpdateTool
	update.UpdateTool = fn
	t.Cleanup(func() { update.UpdateTool = orig })
}

func stubLatestToolVersion(t *testing.T, fn func(tool, installedLabel string) (string, bool, bool)) {
	t.Helper()
	orig := toolupdate.LatestToolVersion
	toolupdate.LatestToolVersion = fn
	t.Cleanup(func() { toolupdate.LatestToolVersion = orig })
}

func stubToolUpdateStdin(t *testing.T, input string) {
	t.Helper()
	orig := toolupdate.Stdin
	toolupdate.Stdin = strings.NewReader(input)
	t.Cleanup(func() { toolupdate.Stdin = orig })
}

func stubToolUpdateIsTerminal(t *testing.T, terminal bool) {
	t.Helper()
	orig := toolupdate.IsTerminal
	toolupdate.IsTerminal = func() bool { return terminal }
	t.Cleanup(func() { toolupdate.IsTerminal = orig })
}

func stubCheckGitAvailable(t *testing.T, err error) {
	t.Helper()
	orig := checkGitAvailable
	checkGitAvailable = func() error { return err }
	t.Cleanup(func() { checkGitAvailable = orig })
}

func stubCurrentGOOS(t *testing.T, goos string) {
	t.Helper()
	orig := currentGOOS
	currentGOOS = goos
	t.Cleanup(func() { currentGOOS = orig })
}

func stubNamespacesStdin(t *testing.T, input string) {
	t.Helper()
	orig := namespacesStdin
	namespacesStdin = strings.NewReader(input)
	t.Cleanup(func() { namespacesStdin = orig })
}

func stubVolumeStdin(t *testing.T, input string) {
	t.Helper()
	orig := volumesStdin
	volumesStdin = strings.NewReader(input)
	t.Cleanup(func() { volumesStdin = orig })
}
