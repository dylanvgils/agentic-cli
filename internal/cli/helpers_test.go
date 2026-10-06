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
	"github.com/dylanvgils/agentic-cli/internal/usecase/toolupdate"
	"github.com/stretchr/testify/require"
)

// fakeDocker implements dockerAPI; a nil field succeeds with a zero value.
type fakeDocker struct {
	context               string
	checkDaemon           func() error
	buildTool             func(tool, image string, opts tools.BuildOptions) error
	updateTool            func(tool, image string, opts tools.BuildOptions) error
	buildProxyImage       func(image, version, sourceDir string, opts tools.BuildOptions) error
	buildDindImage        func(image string, opts tools.BuildOptions) error
	inspectImage          func(string) (*docker.ImageInfo, error)
	listAllImages         func(...docker.ImageFilter) ([]*docker.ImageInfo, error)
	cleanImage            func(string) error
	cleanBaseImages       func() error
	pruneImages           func() error
	pruneBuildCache       func() error
	resolveContainerHome  func(string) string
	runContainer          func(rs docker.RunSpec, toolArgs []string) error
	listRunningContainers func() ([]*docker.ContainerInfo, error)
	ensureNetwork         func() error
	removeNetwork         func() error
	ensureNamedVolumes    func(volumes []string, toolHome, containerHome, chownImage string) error
	createVolume          func(string) error
	listVolumes           func() (string, error)
	listVolumeNames       func() ([]string, error)
	removeVolume          func(string) error
	sweepProxyResources   func() error
	sweepDindResources    func(string) error
	listContexts          func() ([]string, error)
}

func (f *fakeDocker) Context() string { return f.context }

func (f *fakeDocker) CheckDaemon() error {
	if f.checkDaemon == nil {
		return nil
	}
	return f.checkDaemon()
}

func (f *fakeDocker) BuildTool(tool, image string, opts tools.BuildOptions) error {
	if f.buildTool == nil {
		return nil
	}
	return f.buildTool(tool, image, opts)
}

func (f *fakeDocker) UpdateTool(tool, image string, opts tools.BuildOptions) error {
	if f.updateTool == nil {
		return nil
	}
	return f.updateTool(tool, image, opts)
}

func (f *fakeDocker) BuildProxyImage(image, version, sourceDir string, opts tools.BuildOptions) error {
	if f.buildProxyImage == nil {
		return nil
	}
	return f.buildProxyImage(image, version, sourceDir, opts)
}

func (f *fakeDocker) BuildDindImage(image string, opts tools.BuildOptions) error {
	if f.buildDindImage == nil {
		return nil
	}
	return f.buildDindImage(image, opts)
}

func (f *fakeDocker) InspectImage(name string) (*docker.ImageInfo, error) {
	if f.inspectImage == nil {
		return nil, nil
	}
	return f.inspectImage(name)
}

func (f *fakeDocker) ListAllImages(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
	if f.listAllImages == nil {
		return nil, nil
	}
	return f.listAllImages(filters...)
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

func (f *fakeDocker) PruneImages() error {
	if f.pruneImages == nil {
		return nil
	}
	return f.pruneImages()
}

func (f *fakeDocker) PruneBuildCache() error {
	if f.pruneBuildCache == nil {
		return nil
	}
	return f.pruneBuildCache()
}

func (f *fakeDocker) ResolveContainerHome(image string) string {
	if f.resolveContainerHome == nil {
		return ""
	}
	return f.resolveContainerHome(image)
}

func (f *fakeDocker) RunContainer(rs docker.RunSpec, toolArgs []string) error {
	if f.runContainer == nil {
		return nil
	}
	return f.runContainer(rs, toolArgs)
}

func (f *fakeDocker) ListRunningContainers() ([]*docker.ContainerInfo, error) {
	if f.listRunningContainers == nil {
		return nil, nil
	}
	return f.listRunningContainers()
}

func (f *fakeDocker) EnsureNetwork() error {
	if f.ensureNetwork == nil {
		return nil
	}
	return f.ensureNetwork()
}

func (f *fakeDocker) RemoveNetwork() error {
	if f.removeNetwork == nil {
		return nil
	}
	return f.removeNetwork()
}

func (f *fakeDocker) EnsureNamedVolumes(volumes []string, toolHome, containerHome, chownImage string) error {
	if f.ensureNamedVolumes == nil {
		return nil
	}
	return f.ensureNamedVolumes(volumes, toolHome, containerHome, chownImage)
}

func (f *fakeDocker) CreateVolume(name string) error {
	if f.createVolume == nil {
		return nil
	}
	return f.createVolume(name)
}

func (f *fakeDocker) ListVolumes() (string, error) {
	if f.listVolumes == nil {
		return "", nil
	}
	return f.listVolumes()
}

func (f *fakeDocker) ListVolumeNames() ([]string, error) {
	if f.listVolumeNames == nil {
		return nil, nil
	}
	return f.listVolumeNames()
}

func (f *fakeDocker) RemoveVolume(name string) error {
	if f.removeVolume == nil {
		return nil
	}
	return f.removeVolume(name)
}

func (f *fakeDocker) SweepProxyResources() error {
	if f.sweepProxyResources == nil {
		return nil
	}
	return f.sweepProxyResources()
}

func (f *fakeDocker) SweepDindResources(toolHome string) error {
	if f.sweepDindResources == nil {
		return nil
	}
	return f.sweepDindResources(toolHome)
}

func (f *fakeDocker) ListContexts() ([]string, error) {
	if f.listContexts == nil {
		return nil, nil
	}
	return f.listContexts()
}

// overlay copies o's set fields onto f.
func (f *fakeDocker) overlay(o *fakeDocker) {
	if o.context != "" {
		f.context = o.context
	}
	if o.checkDaemon != nil {
		f.checkDaemon = o.checkDaemon
	}
	if o.buildTool != nil {
		f.buildTool = o.buildTool
	}
	if o.updateTool != nil {
		f.updateTool = o.updateTool
	}
	if o.buildProxyImage != nil {
		f.buildProxyImage = o.buildProxyImage
	}
	if o.buildDindImage != nil {
		f.buildDindImage = o.buildDindImage
	}
	if o.inspectImage != nil {
		f.inspectImage = o.inspectImage
	}
	if o.listAllImages != nil {
		f.listAllImages = o.listAllImages
	}
	if o.cleanImage != nil {
		f.cleanImage = o.cleanImage
	}
	if o.cleanBaseImages != nil {
		f.cleanBaseImages = o.cleanBaseImages
	}
	if o.pruneImages != nil {
		f.pruneImages = o.pruneImages
	}
	if o.pruneBuildCache != nil {
		f.pruneBuildCache = o.pruneBuildCache
	}
	if o.resolveContainerHome != nil {
		f.resolveContainerHome = o.resolveContainerHome
	}
	if o.runContainer != nil {
		f.runContainer = o.runContainer
	}
	if o.listRunningContainers != nil {
		f.listRunningContainers = o.listRunningContainers
	}
	if o.ensureNetwork != nil {
		f.ensureNetwork = o.ensureNetwork
	}
	if o.removeNetwork != nil {
		f.removeNetwork = o.removeNetwork
	}
	if o.ensureNamedVolumes != nil {
		f.ensureNamedVolumes = o.ensureNamedVolumes
	}
	if o.createVolume != nil {
		f.createVolume = o.createVolume
	}
	if o.listVolumes != nil {
		f.listVolumes = o.listVolumes
	}
	if o.listVolumeNames != nil {
		f.listVolumeNames = o.listVolumeNames
	}
	if o.removeVolume != nil {
		f.removeVolume = o.removeVolume
	}
	if o.sweepProxyResources != nil {
		f.sweepProxyResources = o.sweepProxyResources
	}
	if o.sweepDindResources != nil {
		f.sweepDindResources = o.sweepDindResources
	}
	if o.listContexts != nil {
		f.listContexts = o.listContexts
	}
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

// stubDocker installs a fakeDocker as dockerClient if the test has none yet, and overlays d's set fields on it until the test ends.
func stubDocker(t *testing.T, d *fakeDocker) {
	t.Helper()

	f, ok := dockerClient.(*fakeDocker)
	if !ok {
		orig := dockerClient
		f = &fakeDocker{}
		dockerClient = f
		t.Cleanup(func() { dockerClient = orig })
	}

	prev := *f
	f.overlay(d)
	t.Cleanup(func() { *f = prev })
}

// restoreDockerClient puts dockerClient back when the test ends, for code under test that rebuilds it.
func restoreDockerClient(t *testing.T) {
	t.Helper()
	orig := dockerClient
	t.Cleanup(func() { dockerClient = orig })
}

// captureRunContainer stubs RunContainer and InspectImage, returning a getter for the captured RunSpec and tool args.
func captureRunContainer(t *testing.T) func() (docker.RunSpec, []string) {
	t.Helper()
	var capturedSpec docker.RunSpec
	var capturedArgs []string

	stubDocker(t, &fakeDocker{
		runContainer: func(rs docker.RunSpec, args []string) error {
			capturedSpec = rs
			capturedArgs = args
			return nil
		},
		inspectImage: func(name string) (*docker.ImageInfo, error) {
			return &docker.ImageInfo{Image: name}, nil
		},
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

func stubBuildProxyImage(t *testing.T, fn func(image, version, sourceDir string, opts tools.BuildOptions) error) {
	t.Helper()
	stubDocker(t, &fakeDocker{buildProxyImage: fn})
}

func stubCheckDockerDaemon(t *testing.T, fn func() error) {
	t.Helper()
	stubDocker(t, &fakeDocker{checkDaemon: fn})
}

func stubMigrateRun(t *testing.T, fn func(string) ([]migrate.Migration, error)) {
	t.Helper()
	orig := migrateRun
	migrateRun = fn
	t.Cleanup(func() { migrateRun = orig })
}

func stubCleanImage(t *testing.T, fn func(string) error) {
	t.Helper()
	stubDocker(t, &fakeDocker{cleanImage: fn})
}

func stubCreateVolume(t *testing.T, fn func(string) error) {
	t.Helper()
	stubDocker(t, &fakeDocker{createVolume: fn})
}

func stubInspectImage(t *testing.T, info *docker.ImageInfo, err error) {
	t.Helper()
	stubDocker(t, &fakeDocker{inspectImage: inspectReturns(info, err)})
}

func stubListAllImages(t *testing.T, fn func(...docker.ImageFilter) ([]*docker.ImageInfo, error)) {
	t.Helper()
	stubDocker(t, &fakeDocker{listAllImages: fn})
}

func stubListRunningContainers(t *testing.T, fn func() ([]*docker.ContainerInfo, error)) {
	t.Helper()
	stubDocker(t, &fakeDocker{listRunningContainers: fn})
}

func stubListVolumeNames(t *testing.T, fn func() ([]string, error)) {
	t.Helper()
	stubDocker(t, &fakeDocker{listVolumeNames: fn})
}

func stubListVolumes(t *testing.T, fn func() (string, error)) {
	t.Helper()
	stubDocker(t, &fakeDocker{listVolumes: fn})
}

func stubListContexts(t *testing.T, fn func() ([]string, error)) {
	t.Helper()
	stubDocker(t, &fakeDocker{listContexts: fn})
}

func stubPruneImages(t *testing.T, fn func() error) {
	t.Helper()
	stubDocker(t, &fakeDocker{pruneImages: fn})
}

func stubPruneBuildCache(t *testing.T, fn func() error) {
	t.Helper()
	stubDocker(t, &fakeDocker{pruneBuildCache: fn})
}

func stubRemoveVolume(t *testing.T, fn func(string) error) {
	t.Helper()
	stubDocker(t, &fakeDocker{removeVolume: fn})
}

func stubPruneProxyLogs(t *testing.T, fn func(dir string, maxAge time.Duration)) {
	t.Helper()
	orig := pruneProxyLogs
	pruneProxyLogs = fn
	t.Cleanup(func() { pruneProxyLogs = orig })
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

func stubBuildDindImage(t *testing.T, fn func(image string, opts tools.BuildOptions) error) {
	t.Helper()
	stubDocker(t, &fakeDocker{buildDindImage: fn})
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

// inspectReturns returns an InspectImage func that always yields info and err.
func inspectReturns(info *docker.ImageInfo, err error) func(string) (*docker.ImageInfo, error) {
	return func(string) (*docker.ImageInfo, error) { return info, err }
}

// formatTestLabelTime formats t like agentic's image timestamp labels.
func formatTestLabelTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z")
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
