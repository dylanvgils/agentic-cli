package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/migrate"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/build"
	"github.com/dylanvgils/agentic-cli/internal/usecase/clean"
	"github.com/dylanvgils/agentic-cli/internal/usecase/run"
	"github.com/dylanvgils/agentic-cli/internal/usecase/toolupdate"
	"github.com/dylanvgils/agentic-cli/internal/usecase/update"
	"github.com/stretchr/testify/require"
)

// fakeCleanDocker implements clean.Docker; a nil field succeeds with a zero value.
type fakeCleanDocker struct {
	listAllImages       func(...docker.ImageFilter) ([]*docker.ImageInfo, error)
	cleanImage          func(string) error
	cleanBaseImages     func() error
	sweepProxyResources func() error
	sweepDindResources  func(string) error
	removeNetwork       func() error
}

func (f *fakeCleanDocker) ListAllImages(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
	if f.listAllImages == nil {
		return nil, nil
	}
	return f.listAllImages(filters...)
}

func (f *fakeCleanDocker) CleanImage(image string) error {
	if f.cleanImage == nil {
		return nil
	}
	return f.cleanImage(image)
}

func (f *fakeCleanDocker) CleanBaseImages() error {
	if f.cleanBaseImages == nil {
		return nil
	}
	return f.cleanBaseImages()
}

func (f *fakeCleanDocker) SweepProxyResources() error {
	if f.sweepProxyResources == nil {
		return nil
	}
	return f.sweepProxyResources()
}

func (f *fakeCleanDocker) SweepDindResources(toolHome string) error {
	if f.sweepDindResources == nil {
		return nil
	}
	return f.sweepDindResources(toolHome)
}

func (f *fakeCleanDocker) RemoveNetwork() error {
	if f.removeNetwork == nil {
		return nil
	}
	return f.removeNetwork()
}

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

// stubErrLog redirects logging.Err to a buffer for the duration of the test and returns it.
func stubErrLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	orig := logging.Err
	logging.Err = logging.New(&buf)
	t.Cleanup(func() { logging.Err = orig })

	return &buf
}

// stubLogs points logging.Log and logging.Err at one buffer, so a test can check ordering across stdout and stderr output.
func stubLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	origLog, origErr := logging.Log, logging.Err
	logging.Log = logging.New(&buf)
	logging.Err = logging.New(&buf)
	t.Cleanup(func() {
		logging.Log = origLog
		logging.Err = origErr
	})

	return &buf
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

	origRunInspect := run.InspectImage
	run.InspectImage = fakeInspect

	t.Cleanup(func() {
		runContainer = origRun
		run.EnsureNamedVolumes = origEnsure
		run.EnsureNetwork = origEnsureNet
		inspectImage = origInspect
		toolupdate.InspectImage = origToolUpdateInspect
		run.InspectImage = origRunInspect
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

func stubMigrateRun(t *testing.T, fn func(string) ([]migrate.Migration, error)) {
	t.Helper()
	orig := migrateRun
	migrateRun = fn
	t.Cleanup(func() { migrateRun = orig })
}

func stubCleanImage(t *testing.T, fn func(string) error) {
	t.Helper()
	orig := cleanImage
	cleanImage = fn
	t.Cleanup(func() { cleanImage = orig })
}

func stubCleanDocker(t *testing.T, d clean.Docker) {
	t.Helper()
	orig := cleanDocker
	cleanDocker = d
	t.Cleanup(func() { cleanDocker = orig })
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

func stubRunInspectImage(t *testing.T, info *docker.ImageInfo, err error) {
	t.Helper()
	orig := run.InspectImage
	run.InspectImage = func(_ string) (*docker.ImageInfo, error) { return info, err }
	t.Cleanup(func() { run.InspectImage = orig })
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

func stubUpdateInspectImageFunc(t *testing.T, fn func(image string) (*docker.ImageInfo, error)) {
	t.Helper()
	orig := update.InspectImage
	update.InspectImage = fn
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

// stubBuildDindImage replaces buildDindImage with fn for the duration of the test.
func stubBuildDindImage(t *testing.T, fn func(image string, opts tools.BuildOptions) error) {
	t.Helper()
	orig := buildDindImage
	buildDindImage = fn
	t.Cleanup(func() { buildDindImage = orig })
}

// formatTestLabelTime formats t like agentic's image timestamp labels.
func formatTestLabelTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

func stubTrustStdin(t *testing.T, input string) {
	t.Helper()
	orig := trustStdin
	trustStdin = strings.NewReader(input)
	t.Cleanup(func() { trustStdin = orig })
}

func stubIsTerminal(t *testing.T, terminal bool) {
	t.Helper()
	orig := isTerminal
	isTerminal = func() bool { return terminal }
	t.Cleanup(func() { isTerminal = orig })
}

// credentialLayer writes a .agenticrc.toml with one credential entry reading secret and returns its layer.
func credentialLayer(t *testing.T, secret string) config.RCLayer {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".agenticrc.toml")
	content := fmt.Sprintf("[[run.proxy.credentials]]\npreset = \"anthropic\"\nsecret = %q\n", secret)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	layers, err := config.FindLayers(filepath.Dir(path))
	require.NoError(t, err)
	return layers[len(layers)-1]
}
