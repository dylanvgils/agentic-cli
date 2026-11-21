package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/microsandbox"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/run"
	"github.com/dylanvgils/agentic-cli/internal/usecase/watch"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// fakeDocker implements dockerAPI with a func field per method a test stubs; a nil field, or a method without one, succeeds with a zero value.
type fakeDocker struct {
	context               string
	checkDaemon           func() error
	buildTool             func(tool, image string, opts tools.BuildOptions) error
	buildProxyImage       func(image, version, sourceDir string, opts tools.BuildOptions) error
	buildDindImage        func(image string, opts tools.BuildOptions) error
	inspectImage          func(string) (*docker.ImageInfo, error)
	listAllImages         func(...docker.ImageFilter) ([]*docker.ImageInfo, error)
	cleanImage            func(string) error
	cleanBaseImages       func() error
	pruneImages           func() (string, error)
	pruneBuildCache       func() (string, error)
	startSidecars         func(rs sandbox.RunSpec, proxyLog string) (docker.Sidecars, error)
	saveImage             func(image string, w io.Writer) error
	listRunningContainers func() ([]*docker.ContainerInfo, error)
	createVolume          func(string) error
	listVolumeNames       func() ([]string, error)
	removeVolume          func(string) error
	listContexts          func() ([]string, error)
	currentContext        func() (string, error)
	version               func() (string, error)
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

func (f *fakeDocker) RestampImage(string, docker.ImageInfo) {}

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

func (f *fakeDocker) PruneImages() (string, error) {
	if f.pruneImages == nil {
		return "", nil
	}
	return f.pruneImages()
}

func (f *fakeDocker) PruneBuildCache() (string, error) {
	if f.pruneBuildCache == nil {
		return "", nil
	}
	return f.pruneBuildCache()
}

func (f *fakeDocker) DryRun(sandbox.RunSpec, string) error {
	return nil
}

func (f *fakeDocker) DryRunPublishedProxy(sandbox.RunSpec, string, sandbox.Runtime) (docker.PublishedProxy, error) {
	return docker.PublishedProxy{}, nil
}

func (f *fakeDocker) StartPublishedProxy(sandbox.RunSpec, string, sandbox.Runtime) (docker.PublishedProxy, error) {
	return docker.PublishedProxy{}, nil
}

func (f *fakeDocker) StartSidecars(rs sandbox.RunSpec, proxyLog string) (docker.Sidecars, error) {
	if f.startSidecars == nil {
		return docker.Sidecars{}, nil
	}
	return f.startSidecars(rs, proxyLog)
}

func (f *fakeDocker) RunTool(docker.Sidecars) error { return nil }

func (f *fakeDocker) StopSidecars(docker.Sidecars) {}

func (f *fakeDocker) SaveImage(image string, w io.Writer) error {
	if f.saveImage == nil {
		return nil
	}
	return f.saveImage(image, w)
}

func (f *fakeDocker) BuildDockerdImage(string, *docker.ImageInfo) error { return nil }

func (f *fakeDocker) ListRunningContainers() ([]*docker.ContainerInfo, error) {
	if f.listRunningContainers == nil {
		return nil, nil
	}
	return f.listRunningContainers()
}

func (f *fakeDocker) RemoveNetwork() error { return nil }

func (f *fakeDocker) EnsureNamedVolumes(sandbox.RunSpec, string) error { return nil }

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

func (f *fakeDocker) SweepRuns(string) (docker.SweptRuns, error) { return docker.SweptRuns{}, nil }

func (f *fakeDocker) Teardown(string, sandbox.RunID) error { return nil }

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

// ServerVersion reports a recent Docker Engine.
func (f *fakeDocker) ServerVersion() (engine, version string, err error) {
	return "Engine", "29.0.0", nil
}

func (f *fakeDocker) RequireLocalContext(string) error { return nil }

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
	if o.startSidecars != nil {
		f.startSidecars = o.startSidecars
	}
	if o.saveImage != nil {
		f.saveImage = o.saveImage
	}
	if o.listRunningContainers != nil {
		f.listRunningContainers = o.listRunningContainers
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
	if o.listContexts != nil {
		f.listContexts = o.listContexts
	}
	if o.currentContext != nil {
		f.currentContext = o.currentContext
	}
	if o.version != nil {
		f.version = o.version
	}
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

// captureToolRun stubs StartSidecars and InspectImage, returning a getter for the captured RunSpec.
func captureToolRun(t *testing.T) func() sandbox.RunSpec {
	t.Helper()
	var capturedSpec sandbox.RunSpec

	stubDocker(t, &fakeDocker{
		startSidecars: func(rs sandbox.RunSpec, _ string) (docker.Sidecars, error) {
			capturedSpec = rs
			return docker.Sidecars{}, nil
		},
		inspectImage: func(name string) (*docker.ImageInfo, error) {
			return &docker.ImageInfo{Image: name}, nil
		},
	})

	return func() sandbox.RunSpec { return capturedSpec }
}

// fakeMicrosandbox is a hand-written msbAPI with a func field per method a test stubs; nil funcs and methods without one succeed.
type fakeMicrosandbox struct {
	available       func() error
	loadImage       func(r io.Reader, tag string) error
	run             func(rs sandbox.RunSpec) error
	cleanImage      func(image string) error
	listRunning     func() ([]*microsandbox.Sandbox, error)
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

// HasImage reports every ref missing.
func (f *fakeMicrosandbox) HasImage(string) (bool, error) { return false, nil }

func (f *fakeMicrosandbox) LoadImage(r io.Reader, tag string) error {
	if f.loadImage == nil {
		return nil
	}
	return f.loadImage(r, tag)
}

func (f *fakeMicrosandbox) ImageRefs() ([]string, error) { return nil, nil }

func (f *fakeMicrosandbox) RemoveImage(string) error { return nil }

func (f *fakeMicrosandbox) Run(rs sandbox.RunSpec) error {
	if f.run == nil {
		return nil
	}
	return f.run(rs)
}

func (f *fakeMicrosandbox) DryRun(sandbox.RunSpec) error {
	return nil
}

func (f *fakeMicrosandbox) DryRunThroughProxy(sandbox.RunSpec, microsandbox.ProxyRoute) error {
	return nil
}

func (f *fakeMicrosandbox) RunThroughProxy(rs sandbox.RunSpec, _ microsandbox.ProxyRoute) error {
	return f.Run(rs)
}

func (f *fakeMicrosandbox) Teardown(sandbox.RunID) error { return nil }

func (f *fakeMicrosandbox) CleanImage(image string) error {
	if f.cleanImage == nil {
		return nil
	}
	return f.cleanImage(image)
}

func (f *fakeMicrosandbox) ListRunning() ([]*microsandbox.Sandbox, error) {
	if f.listRunning == nil {
		return nil, nil
	}
	return f.listRunning()
}

func (f *fakeMicrosandbox) Ports(string) ([]microsandbox.PublishedPort, error) { return nil, nil }

// Version reports a recent msb unless stubbed.
func (f *fakeMicrosandbox) Version() (string, error) {
	if f.version == nil {
		return "0.7.12", nil
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

// writeProxyLog writes a proxy access log in a run log dir under the current toolHome and returns its path.
func writeProxyLog(t *testing.T) string {
	t.Helper()
	dir := logging.Dir(toolHome, "0123456789ab", time.Now())
	require.NoError(t, os.MkdirAll(dir, 0o750))
	path := filepath.Join(dir, proxy.LogFileName)
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

func stubCheckDockerDaemon(t *testing.T, fn func() error) {
	t.Helper()
	stubDocker(t, &fakeDocker{checkDaemon: fn})
}

func stubCleanImage(t *testing.T, fn func(string) error) {
	t.Helper()
	stubDocker(t, &fakeDocker{cleanImage: fn})
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

func stubRemoveVolume(t *testing.T, fn func(string) error) {
	t.Helper()
	stubDocker(t, &fakeDocker{removeVolume: fn})
}

// stubUpstream makes upstreamClient report latest, newer and ok for every tool for the duration of the test.
func stubUpstream(t *testing.T, latest string, newer, ok bool) {
	t.Helper()
	orig := upstreamClient
	upstreamClient = fakeUpstream{latest: latest, newer: newer, ok: ok}
	t.Cleanup(func() { upstreamClient = orig })
}

// fakeUpstream implements update.Upstream with a fixed answer.
type fakeUpstream struct {
	latest string
	newer  bool
	ok     bool
}

func (f fakeUpstream) LatestToolVersion(string, string) (string, bool, bool) {
	return f.latest, f.newer, f.ok
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
	addAllFlag(cmd, "")
	return cmd
}

func newTestPruneCmd(force, logs bool) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("force", force, "")
	cmd.Flags().Bool("logs", logs, "")
	return cmd
}

// stubBuildTool stubs Docker with info as the built image and returns the options BuildTool last got.
func stubBuildTool(t *testing.T, info *docker.ImageInfo) *tools.BuildOptions {
	t.Helper()
	var opts tools.BuildOptions
	stubDocker(t, &fakeDocker{
		buildTool: func(_, _ string, o tools.BuildOptions) error {
			opts = o
			return nil
		},
		inspectImage: inspectReturns(info, nil),
	})
	return &opts
}

// newTestAllCmd returns a cmd with the --all flag, set when all.
func newTestAllCmd(t *testing.T, all bool) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	addAllFlag(cmd, "")
	if all {
		require.NoError(t, cmd.Flags().Set(allFlagName, "true"))
	}
	return cmd
}

// writeRC writes content as dir's .agenticrc.toml.
func writeRC(t *testing.T, dir, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".agenticrc.toml"), []byte(content), 0o644))
}

// newTestPsCmd returns a cmd with ps's flags, set to args.
func newTestPsCmd(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	addNamespaceFlag(cmd)
	addAllFlag(cmd, "")
	require.NoError(t, cmd.ParseFlags(args))
	return cmd
}

// newTestWatchCmd returns a cmd with the watchdog's flags and the root's --docker-context, set to args.
func newTestWatchCmd(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().String(watch.FlagRun, "", "")
	cmd.Flags().String(watch.FlagRuntime, "", "")
	cmd.Flags().String(watch.FlagTool, "", "")
	cmd.Flags().String(dockerContextFlagName, "", "")
	cmd.Flags().Int(watch.FlagPID, 0, "")
	require.NoError(t, cmd.ParseFlags(args))
	return cmd
}

// stubReleases makes releasesClient f for the duration of the test.
func stubReleases(t *testing.T, f *fakeReleases) {
	t.Helper()
	orig := releasesClient
	releasesClient = f
	t.Cleanup(func() { releasesClient = orig })
}

// fakeReleases implements upgradecheck.Releases, returning latest/latestErr and updateErr and recording the calls.
type fakeReleases struct {
	latest    string
	latestErr error
	updateErr error

	fetched     bool
	updatedWith string
}

func (f *fakeReleases) LatestVersion() (string, error) {
	f.fetched = true
	return f.latest, f.latestErr
}

func (f *fakeReleases) Update(version string) error {
	f.updatedWith = version
	return f.updateErr
}

// stubToolHome points toolHome at dir.
func stubToolHome(t *testing.T, dir string) {
	t.Helper()
	orig := toolHome
	toolHome = dir
	t.Cleanup(func() { toolHome = orig })
}

// fakeWatchdogs implements watchdogStarter, recording each Docker context and spawned run instead of starting a process.
type fakeWatchdogs struct {
	contexts []string
	spawned  []sandbox.RunRef
}

func (f *fakeWatchdogs) For(dockerContext string) run.Watchdog {
	f.contexts = append(f.contexts, dockerContext)
	return f
}

func (f *fakeWatchdogs) Spawn(r sandbox.RunRef) error {
	f.spawned = append(f.spawned, r)
	return nil
}

// stubWatchdogs makes watchdogs a fresh fakeWatchdogs for the duration of the test.
func stubWatchdogs(t *testing.T) *fakeWatchdogs {
	t.Helper()
	orig := watchdogs
	f := &fakeWatchdogs{}
	watchdogs = f
	t.Cleanup(func() { watchdogs = orig })
	return f
}
