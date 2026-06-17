package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/require"
)

// captureStdout replaces os.Stdout with a pipe and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = orig
	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck
	return buf.String()
}

// captureRunContainer replaces runContainer, ensureNamedVolumes, ensureNetwork,
// and inspectImage with stubs that record the RunSpec and tool args. Returns a
// getter for the captured values.
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

	origEnsure := ensureNamedVolumes
	ensureNamedVolumes = func(volumes []string, toolHome, containerHome, chownImage string) error {
		return nil
	}

	origEnsureNet := ensureNetwork
	ensureNetwork = func() error { return nil }

	origInspect := inspectImage
	inspectImage = func(name string) (*docker.ImageInfo, error) {
		return &docker.ImageInfo{Image: name}, nil
	}

	t.Cleanup(func() {
		runContainer = origRun
		ensureNamedVolumes = origEnsure
		ensureNetwork = origEnsureNet
		inspectImage = origInspect
	})

	return func() (docker.RunSpec, []string) { return capturedSpec, capturedArgs }
}

// withTempToolHome sets toolHome to a temp dir and pre-trusts the directories
// that tests run in (os.TempDir() covers t.Chdir paths; cwd covers tests that
// don't chdir).
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
	orig := buildTool
	buildTool = fn
	t.Cleanup(func() { buildTool = orig })
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

func stubCleanBaseImages(t *testing.T, fn func() error) {
	t.Helper()
	orig := cleanBaseImages
	cleanBaseImages = fn
	t.Cleanup(func() { cleanBaseImages = orig })
}

func stubCleanImage(t *testing.T, fn func(string) error) {
	t.Helper()
	orig := cleanImage
	cleanImage = fn
	t.Cleanup(func() { cleanImage = orig })
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

func stubRemoveNetwork(t *testing.T, fn func() error) {
	t.Helper()
	orig := removeNetwork
	removeNetwork = fn
	t.Cleanup(func() { removeNetwork = orig })
}

func stubRemoveVolume(t *testing.T, fn func(string) error) {
	t.Helper()
	orig := removeVolume
	removeVolume = fn
	t.Cleanup(func() { removeVolume = orig })
}

func stubSweepProxyResources(t *testing.T, fn func() error) {
	t.Helper()
	orig := sweepProxyResources
	sweepProxyResources = fn
	t.Cleanup(func() { sweepProxyResources = orig })
}

func stubUpdateTool(t *testing.T, fn func(tool, image string, opts tools.BuildOptions) error) {
	t.Helper()
	orig := updateTool
	updateTool = fn
	t.Cleanup(func() { updateTool = orig })
}

func stubEnsureNamedVolumes(t *testing.T, fn func(volumes []string, toolHome, containerHome, chownImage string) error) {
	t.Helper()
	orig := ensureNamedVolumes
	ensureNamedVolumes = fn
	t.Cleanup(func() { ensureNamedVolumes = orig })
}

func stubEnsureNetwork(t *testing.T, fn func() error) {
	t.Helper()
	orig := ensureNetwork
	ensureNetwork = fn
	t.Cleanup(func() { ensureNetwork = orig })
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
