package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/microsandbox"
	"github.com/dylanvgils/agentic-cli/internal/migrate"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/update"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// fakeDocker implements dockerAPI; a nil field succeeds with a zero value.
type fakeDocker struct {
	context               string
	checkDaemon           func() error
	buildTool             func(tool, image string, opts tools.BuildOptions) error
	restampImage          func(image string, info docker.ImageInfo)
	buildProxyImage       func(image, version, sourceDir string, opts tools.BuildOptions) error
	buildDindImage        func(image string, opts tools.BuildOptions) error
	inspectImage          func(string) (*docker.ImageInfo, error)
	listAllImages         func(...docker.ImageFilter) ([]*docker.ImageInfo, error)
	cleanImage            func(string) error
	cleanBaseImages       func() error
	pruneDangling         func() error
	resolveContainerHome  func(string) string
	startSidecars         func(rs sandbox.RunSpec) (docker.Sidecars, error)
	runTool               func(sidecars docker.Sidecars, toolArgs []string) error
	stopSidecars          func(sidecars docker.Sidecars)
	saveImage             func(image string, w io.Writer) error
	listRunningContainers func() ([]*docker.ContainerInfo, error)
	removeNetwork         func() error
	ensureNamedVolumes    func(volumes []string, toolHome, containerHome, chownImage string) error
	createVolume          func(string) error
	listVolumeNames       func() ([]string, error)
	removeVolume          func(string) error
	sweepProxyResources   func() error
	sweepDindResources    func(string) error
	sweepRunNetworks      func() error
	listContexts          func() ([]string, error)
	currentContext        func() (string, error)
	version               func() (string, error)
	serverVersion         func() (engine, version string, err error)
}

func (f *fakeDocker) Context() string { return f.context }

// Capabilities returns Docker's real capabilities, which tests have no reason to change.
func (f *fakeDocker) Capabilities() sandbox.Capabilities {
	return (&docker.Client{}).Capabilities()
}

func (f *fakeDocker) CurrentContext() (string, error) {
	if f.currentContext == nil {
		return f.context, nil
	}
	return f.currentContext()
}

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

func (f *fakeDocker) RestampImage(image string, info docker.ImageInfo) {
	if f.restampImage != nil {
		f.restampImage(image, info)
	}
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

func (f *fakeMicrosandbox) ListVolumeNames() ([]string, error) {
	if f.listVolumeNames == nil {
		return nil, nil
	}
	return f.listVolumeNames()
}

func (f *fakeMicrosandbox) RemoveVolume(name string) error {
	if f.removeVolume == nil {
		return nil
	}
	return f.removeVolume(name)
}

func (f *fakeDocker) CleanBaseImages() error {
	if f.cleanBaseImages == nil {
		return nil
	}
	return f.cleanBaseImages()
}

func (f *fakeDocker) PruneDangling() error {
	if f.pruneDangling == nil {
		return nil
	}
	return f.pruneDangling()
}

func (f *fakeDocker) ResolveContainerHome(image string) string {
	if f.resolveContainerHome == nil {
		return ""
	}
	return f.resolveContainerHome(image)
}

func (f *fakeDocker) StartSidecars(rs sandbox.RunSpec) (docker.Sidecars, error) {
	if f.startSidecars == nil {
		return docker.Sidecars{}, nil
	}
	return f.startSidecars(rs)
}

func (f *fakeDocker) RunTool(sidecars docker.Sidecars, toolArgs []string) error {
	if f.runTool == nil {
		return nil
	}
	return f.runTool(sidecars, toolArgs)
}

func (f *fakeDocker) StopSidecars(sidecars docker.Sidecars) {
	if f.stopSidecars != nil {
		f.stopSidecars(sidecars)
	}
}

func (f *fakeDocker) SaveImage(image string, w io.Writer) error {
	if f.saveImage == nil {
		return nil
	}
	return f.saveImage(image, w)
}

func (f *fakeDocker) ListRunningContainers() ([]*docker.ContainerInfo, error) {
	if f.listRunningContainers == nil {
		return nil, nil
	}
	return f.listRunningContainers()
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

func (f *fakeDocker) SweepRunNetworks() error {
	if f.sweepRunNetworks == nil {
		return nil
	}
	return f.sweepRunNetworks()
}

func (f *fakeDocker) ListContexts() ([]string, error) {
	if f.listContexts == nil {
		return nil, nil
	}
	return f.listContexts()
}

func (f *fakeDocker) Version() (string, error) {
	if f.version == nil {
		return "", nil
	}
	return f.version()
}

// ServerVersion reports a recent Docker Engine unless stubbed.
func (f *fakeDocker) ServerVersion() (engine, version string, err error) {
	if f.serverVersion == nil {
		return "Engine", "29.0.0", nil
	}
	return f.serverVersion()
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
	if o.restampImage != nil {
		f.restampImage = o.restampImage
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
	if o.pruneDangling != nil {
		f.pruneDangling = o.pruneDangling
	}
	if o.resolveContainerHome != nil {
		f.resolveContainerHome = o.resolveContainerHome
	}
	if o.startSidecars != nil {
		f.startSidecars = o.startSidecars
	}
	if o.runTool != nil {
		f.runTool = o.runTool
	}
	if o.stopSidecars != nil {
		f.stopSidecars = o.stopSidecars
	}
	if o.saveImage != nil {
		f.saveImage = o.saveImage
	}
	if o.listRunningContainers != nil {
		f.listRunningContainers = o.listRunningContainers
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
	if o.sweepRunNetworks != nil {
		f.sweepRunNetworks = o.sweepRunNetworks
	}
	if o.listContexts != nil {
		f.listContexts = o.listContexts
	}
	if o.currentContext != nil {
		f.currentContext = o.currentContext
	}
	if o.version != nil {
		f.version = o.version
	}
	if o.serverVersion != nil {
		f.serverVersion = o.serverVersion
	}
}

// captureStdout replaces os.Stdout with a pipe and returns what was written; for logging.Step/Detail-based output, use captureLog instead.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stdout
	os.Stdout = w
	// Restore even when fn fails a require and exits the test early
	t.Cleanup(func() { os.Stdout = orig })
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

// captureToolRun stubs StartSidecars, RunTool and InspectImage, returning a getter for the captured RunSpec and tool args.
func captureToolRun(t *testing.T) func() (sandbox.RunSpec, []string) {
	t.Helper()
	var capturedSpec sandbox.RunSpec
	var capturedArgs []string

	stubDocker(t, &fakeDocker{
		startSidecars: func(rs sandbox.RunSpec) (docker.Sidecars, error) {
			capturedSpec = rs
			return docker.Sidecars{}, nil
		},
		runTool: func(_ docker.Sidecars, args []string) error {
			capturedArgs = args
			return nil
		},
		inspectImage: func(name string) (*docker.ImageInfo, error) {
			return &docker.ImageInfo{Image: name}, nil
		},
	})

	return func() (sandbox.RunSpec, []string) { return capturedSpec, capturedArgs }
}

// fakeMicrosandbox is a hand-written msbAPI; nil funcs succeed.
type fakeMicrosandbox struct {
	available       func() error
	loadImage       func(r io.Reader, tag string) error
	imageRefs       func() ([]string, error)
	removeImage     func(ref string) error
	run             func(rs sandbox.RunSpec, toolArgs []string) error
	cleanImage      func(image string) error
	listVolumeNames func() ([]string, error)
	removeVolume    func(name string) error
	version         func() (string, error)
}

// Capabilities returns msb's real capabilities, which tests have no reason to change.
func (f *fakeMicrosandbox) Capabilities() sandbox.Capabilities {
	return (&microsandbox.Client{}).Capabilities()
}

func (f *fakeMicrosandbox) Available() error {
	if f.available == nil {
		return nil
	}
	return f.available()
}

func (f *fakeMicrosandbox) LoadImage(r io.Reader, tag string) error {
	if f.loadImage == nil {
		return nil
	}
	return f.loadImage(r, tag)
}

func (f *fakeMicrosandbox) ImageRefs() ([]string, error) {
	if f.imageRefs == nil {
		return nil, nil
	}
	return f.imageRefs()
}

func (f *fakeMicrosandbox) RemoveImage(ref string) error {
	if f.removeImage == nil {
		return nil
	}
	return f.removeImage(ref)
}

func (f *fakeMicrosandbox) Run(rs sandbox.RunSpec, toolArgs []string) error {
	if f.run == nil {
		return nil
	}
	return f.run(rs, toolArgs)
}

func (f *fakeMicrosandbox) CleanImage(image string) error {
	if f.cleanImage == nil {
		return nil
	}
	return f.cleanImage(image)
}

// Version reports a recent msb unless stubbed.
func (f *fakeMicrosandbox) Version() (string, error) {
	if f.version == nil {
		return "0.7.8", nil
	}
	return f.version()
}

// stubMicrosandbox installs f as msbClient until the test ends.
func stubMicrosandbox(t *testing.T, f *fakeMicrosandbox) {
	t.Helper()
	orig := msbClient
	msbClient = f
	t.Cleanup(func() { msbClient = orig })
}

// writeProxyLog writes a proxy access log under the current toolHome's logs dir and returns its path.
func writeProxyLog(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(toolHome, config.LogsDirName)
	require.NoError(t, os.MkdirAll(dir, 0o750))
	path := filepath.Join(dir, proxy.LogFilePrefix+"run.jsonl")
	require.NoError(t, os.WriteFile(path, []byte("{}\n"), 0o600))
	return path
}

// withTempToolHome sets toolHome to a temp dir and pre-trusts the dirs tests run in (os.TempDir() for t.Chdir, cwd otherwise).
func withTempToolHome(t *testing.T) {
	t.Helper()
	homeDir := t.TempDir()
	cwd, err := os.Getwd()
	require.NoError(t, err)
	var trusted []string
	for _, dir := range []string{os.TempDir(), cwd} {
		// Real paths, since trusted entries compare as stored (macOS's /var is a link)
		resolved, err := filepath.EvalSymlinks(dir)
		require.NoError(t, err)
		trusted = append(trusted, resolved)
	}
	cfg := &config.State{TrustedDirs: trusted}
	require.NoError(t, cfg.Save(homeDir))
	orig := toolHome
	toolHome = homeDir
	t.Cleanup(func() { toolHome = orig })
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

func stubListContexts(t *testing.T, fn func() ([]string, error)) {
	t.Helper()
	stubDocker(t, &fakeDocker{listContexts: fn})
}

func stubCurrentContext(t *testing.T, fn func() (string, error)) {
	t.Helper()
	stubDocker(t, &fakeDocker{currentContext: fn})
}

func stubRemoveVolume(t *testing.T, fn func(string) error) {
	t.Helper()
	stubDocker(t, &fakeDocker{removeVolume: fn})
}

func stubUpdateLatestToolVersion(t *testing.T, latest string, newer, ok bool) {
	t.Helper()
	orig := update.LatestToolVersion
	update.LatestToolVersion = func(string, string) (string, bool, bool) { return latest, newer, ok }
	t.Cleanup(func() { update.LatestToolVersion = orig })
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

func stubBuildDindImage(t *testing.T, fn func(image string, opts tools.BuildOptions) error) {
	t.Helper()
	stubDocker(t, &fakeDocker{buildDindImage: fn})
}

// stubStdin makes every confirmation prompt read input for the duration of the test.
func stubStdin(t *testing.T, input string) {
	t.Helper()
	orig := stdin
	stdin = strings.NewReader(input)
	t.Cleanup(func() { stdin = orig })
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

// stubFlag sets cmd's flag name to value for the duration of the test, then restores its default and Changed state.
func stubFlag(t *testing.T, cmd *cobra.Command, name, value string) {
	t.Helper()
	flag := cmd.Flags().Lookup(name)
	require.NotNil(t, flag, "unknown flag %q", name)
	require.NoError(t, cmd.Flags().Set(name, value))

	t.Cleanup(func() {
		// A slice flag appends on every Set after the first, so replace its value instead
		if slice, ok := flag.Value.(interface{ Replace([]string) error }); ok {
			slice.Replace(sliceDefault(flag.DefValue)) //nolint:errcheck
		} else {
			flag.Value.Set(flag.DefValue) //nolint:errcheck
		}
		flag.Changed = false
	})
}

// sliceDefault parses a slice flag's DefValue, e.g. "[a,b]", back into its values.
func sliceDefault(defValue string) []string {
	inner := strings.Trim(defValue, "[]")
	if inner == "" {
		return nil
	}
	return strings.Split(inner, ",")
}

func newTestCleanCmd() *cobra.Command {
	cmd := &cobra.Command{}
	addNamespaceFlag(cmd)
	addAllFlag(cmd)
	return cmd
}

// writeMarketplaceRC writes a minimal .agenticrc.toml declaring one marketplace into dir.
func writeMarketplaceRC(t *testing.T, dir, name, url string) {
	t.Helper()
	content := "root = true\n\n[[marketplaces]]\nname = \"" + name + "\"\nurl = \"" + url + "\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".agenticrc.toml"), []byte(content), 0o644))
}

func newTestStatusCmd(t *testing.T) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	return cmd, &buf
}

func stubFetchLatestVersion(t *testing.T, v string, err error) {
	t.Helper()
	orig := fetchLatestVersion
	fetchLatestVersion = func() (string, error) { return v, err }
	t.Cleanup(func() { fetchLatestVersion = orig })
}

func stubPerformUpdate(t *testing.T, err error) {
	t.Helper()
	orig := performUpdate
	performUpdate = func(_ string) error { return err }
	t.Cleanup(func() { performUpdate = orig })
}

// stubToolHome points toolHome at dir.
func stubToolHome(t *testing.T, dir string) {
	t.Helper()
	orig := toolHome
	toolHome = dir
	t.Cleanup(func() { toolHome = orig })
}
