package run

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/microsandbox"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/require"
)

// fakeDocker implements Docker; a nil field succeeds with a zero value, except capabilities, which defaults to Docker's.
type fakeDocker struct {
	capabilities         func() sandbox.Capabilities
	inspectImage         func(string) (*docker.ImageInfo, error)
	resolveContainerHome func(string) string
	ensureNamedVolumes   func(rs sandbox.RunSpec, chownImage string) error
	listAllImages        func(...docker.ImageFilter) ([]*docker.ImageInfo, error)
	buildProxyImage      func(image, version, sourceDir string, opts tools.BuildOptions) error
	buildDindImage       func(image string, opts tools.BuildOptions) error
	buildTool            func(tool, image string, opts tools.BuildOptions) error
	restampImage         func(image string, info docker.ImageInfo)
	pruneImages          func() (string, error)
	saveImage            func(image string, w io.Writer) error
	buildDockerdImage    func(image string, parent *docker.ImageInfo) error
	startSidecars        func(rs sandbox.RunSpec) (docker.Sidecars, error)
	runTool              func(sidecars docker.Sidecars) error
	stopSidecars         func(sidecars docker.Sidecars)
	serverVersion        func() (engine, version string, err error)
}

func (f *fakeDocker) Capabilities() sandbox.Capabilities {
	if f.capabilities == nil {
		return (&docker.Client{}).Capabilities()
	}
	return f.capabilities()
}

// ServerVersion reports a Docker Engine at the minimum unless stubbed.
func (f *fakeDocker) ServerVersion() (engine, version string, err error) {
	if f.serverVersion == nil {
		return dockerEngineName, minDockerEngine, nil
	}
	return f.serverVersion()
}

func (f *fakeDocker) BuildDockerdImage(image string, parent *docker.ImageInfo) error {
	if f.buildDockerdImage == nil {
		return nil
	}
	return f.buildDockerdImage(image, parent)
}

func (f *fakeDocker) InspectImage(name string) (*docker.ImageInfo, error) {
	if f.inspectImage == nil {
		return nil, nil
	}
	return f.inspectImage(name)
}

func (f *fakeDocker) ResolveContainerHome(image string) string {
	if f.resolveContainerHome == nil {
		return ""
	}
	return f.resolveContainerHome(image)
}

func (f *fakeDocker) EnsureNamedVolumes(rs sandbox.RunSpec, chownImage string) error {
	if f.ensureNamedVolumes == nil {
		return nil
	}
	return f.ensureNamedVolumes(rs, chownImage)
}

func (f *fakeDocker) ListAllImages(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
	if f.listAllImages == nil {
		return nil, nil
	}
	return f.listAllImages(filters...)
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

func (f *fakeDocker) PruneImages() (string, error) {
	if f.pruneImages == nil {
		return "", nil
	}
	return f.pruneImages()
}

func (f *fakeDocker) SaveImage(image string, w io.Writer) error {
	if f.saveImage == nil {
		return nil
	}
	return f.saveImage(image, w)
}

func (f *fakeDocker) StartSidecars(rs sandbox.RunSpec) (docker.Sidecars, error) {
	if f.startSidecars == nil {
		return docker.Sidecars{}, nil
	}
	return f.startSidecars(rs)
}

func (f *fakeDocker) RunTool(sidecars docker.Sidecars) error {
	if f.runTool == nil {
		return nil
	}
	return f.runTool(sidecars)
}

func (f *fakeDocker) StopSidecars(sidecars docker.Sidecars) {
	if f.stopSidecars != nil {
		f.stopSidecars(sidecars)
	}
}

// newProxyRunSpec returns a RunSpec with the proxy in mode and its log file in a temp tool home, as Build sets it.
func newProxyRunSpec(t *testing.T, mode sandbox.ProxyMode) sandbox.RunSpec {
	t.Helper()
	home := t.TempDir()
	logFile, err := proxyLogFile(home, true)
	require.NoError(t, err)
	return sandbox.RunSpec{ToolHome: home, Proxy: sandbox.ProxySpec{Mode: mode, LogFile: logFile}}
}

// runtimeRequest returns a Request that runs on runtime.
func runtimeRequest(runtime sandbox.Runtime) Request {
	return Request{Project: Project{RC: &config.AgenticRC{}}, Flags: Flags{Runtime: runtime}}
}

// inspectImageID returns an InspectImage func reporting every image with id.
func inspectImageID(id string) func(string) (*docker.ImageInfo, error) {
	return func(name string) (*docker.ImageInfo, error) {
		return &docker.ImageInfo{Image: name, ID: id}, nil
	}
}

// fakeMicrosandbox implements Microsandbox; a nil field succeeds, except capabilities, which defaults to msb's.
type fakeMicrosandbox struct {
	capabilities func() sandbox.Capabilities
	available    func() error
	version      func() (string, error)
	loadImage    func(r io.Reader, tag string) error
	imageRefs    func() ([]string, error)
	removeImage  func(ref string) error
	run          func(rs sandbox.RunSpec) error
}

func (f *fakeMicrosandbox) Capabilities() sandbox.Capabilities {
	if f.capabilities == nil {
		return (&microsandbox.Client{}).Capabilities()
	}
	return f.capabilities()
}

func (f *fakeMicrosandbox) Available() error {
	if f.available == nil {
		return nil
	}
	return f.available()
}

// Version reports msb at the minimum unless stubbed.
func (f *fakeMicrosandbox) Version() (string, error) {
	if f.version == nil {
		return minMicrosandbox, nil
	}
	return f.version()
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

func (f *fakeMicrosandbox) Run(rs sandbox.RunSpec) error {
	if f.run == nil {
		return nil
	}
	return f.run(rs)
}

// fakePrompter implements Prompter, recording what it was asked; a nil field approves, except tool updates, which it declines.
type fakePrompter struct {
	trustDir        func(string) error
	approveSettings func(config.RCLayer, []config.GuardedSetting) error
	offerToolUpdate func(tool, installed, latest string) bool

	trustAsked     []string
	settingsAsked  [][]string
	updatesOffered []string
}

func (f *fakePrompter) TrustDir(dir string) error {
	f.trustAsked = append(f.trustAsked, dir)
	if f.trustDir == nil {
		return nil
	}
	return f.trustDir(dir)
}

func (f *fakePrompter) ApproveSettings(layer config.RCLayer, changed []config.GuardedSetting) error {
	var keys []string
	for _, setting := range changed {
		keys = append(keys, setting.Key)
	}
	f.settingsAsked = append(f.settingsAsked, keys)
	if f.approveSettings == nil {
		return nil
	}
	return f.approveSettings(layer, changed)
}

func (f *fakePrompter) OfferToolUpdate(tool, installed, latest string) bool {
	f.updatesOffered = append(f.updatesOffered, tool)
	if f.offerToolUpdate == nil {
		return false
	}
	return f.offerToolUpdate(tool, installed, latest)
}

// symlinkWorkspace is a cwd with real/, link -> real and escape -> outside.
type symlinkWorkspace struct {
	dir     string
	real    string
	outside string
}

// findVolumeSuffix returns the one volume spec ending with suffix, failing the test if there isn't exactly one match.
func findVolumeSuffix(t *testing.T, volumes []string, suffix string) string {
	t.Helper()
	var matches []string
	for _, v := range volumes {
		if strings.HasSuffix(v, suffix) {
			matches = append(matches, v)
		}
	}
	require.Len(t, matches, 1, "expected exactly one volume ending with %s, got %v", suffix, volumes)
	return matches[0]
}

// appendFile appends content to the file at path, simulating a tool writing notes below the agentic-managed block at runtime.
func appendFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, err = f.WriteString(content)
	return err
}

// credentialLayer returns a config layer declaring one generic credential reading secret.
func credentialLayer(t *testing.T, secret string) config.RCLayer {
	t.Helper()
	rc := &config.AgenticRC{}
	rc.Run.Proxy.Credentials = []config.RCCredential{{Hosts: []string{"api.example.test"}, Header: "X-Api-Key", Env: []string{"EXAMPLE_API_KEY"}, Secret: secret}}
	return config.RCLayer{Path: filepath.Join(t.TempDir(), ".agenticrc.toml"), RC: rc}
}

// approveSettings records layer's guarded settings as approved in toolHome's state.json.
func approveSettings(t *testing.T, layer config.RCLayer, toolHome string) {
	t.Helper()
	cfg, err := config.LoadState(toolHome)
	require.NoError(t, err)
	require.NoError(t, cfg.ApproveSettings(layer, toolHome))
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

// newPrepareRequest returns a Request for claude in a fresh home and cwd, with the tool update check disabled.
func newPrepareRequest(t *testing.T) Request {
	t.Helper()
	checkUpdates := false
	rc := &config.AgenticRC{}
	rc.Run.CheckUpdates = &checkUpdates

	return Request{
		Target:  Target{ToolName: "claude", ImageName: "agentic-claude"},
		Tool:    tools.Configs["claude"],
		Project: Project{Dir: t.TempDir(), RC: rc},
		Flags:   Flags{ToolHome: t.TempDir()},
	}
}

// newCredentialRequest returns newPrepareRequest with one proxy credential declared, reading a test-secret file.
func newCredentialRequest(t *testing.T) Request {
	t.Helper()
	req := newPrepareRequest(t)
	secret := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(secret, []byte("test-secret\n"), 0o600))
	layer := credentialLayer(t, secret)
	req.Project.Layers = []config.RCLayer{layer}
	req.Project.RC.Run.Proxy.Credentials = layer.RC.Run.Proxy.Credentials
	return req
}

// withLatestToolVersion makes svc's tool update check see latest as newer.
func withLatestToolVersion(svc *Service, latest string) *Service {
	svc.upstream = fakeUpstream(latest)
	return svc
}

// fakeUpstream implements toolupdate.Upstream, reporting itself as a newer latest version.
type fakeUpstream string

func (f fakeUpstream) LatestToolVersion(string, string) (string, bool, bool) {
	return string(f), true, true
}

// stubLog redirects logging.Log to a buffer for the duration of the test and returns it.
func stubLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := logging.Log
	logging.Log = logging.New(&buf)
	t.Cleanup(func() { logging.Log = orig })
	return &buf
}

// chdirSymlinkWorkspace makes a fresh symlinkWorkspace the cwd.
func chdirSymlinkWorkspace(t *testing.T) symlinkWorkspace {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}

	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	outside, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	t.Chdir(dir)

	realDir := filepath.Join(dir, "real")
	require.NoError(t, os.MkdirAll(filepath.Join(realDir, "sub"), 0o700))
	require.NoError(t, os.Symlink(realDir, filepath.Join(dir, "link")))
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "escape")))
	return symlinkWorkspace{dir: dir, real: realDir, outside: outside}
}

// stubUnresolvablePath returns a path through a link inside a dir agentic can't search, like /proc/1/root for a non-root user.
func stubUnresolvablePath(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs unix permissions that bind the current user")
	}

	locked := filepath.Join(t.TempDir(), "locked")
	require.NoError(t, os.Mkdir(locked, 0o700))
	require.NoError(t, os.Symlink("/", filepath.Join(locked, "root")))
	require.NoError(t, os.Chmod(locked, 0))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	return filepath.Join(locked, "root", "home")
}

// newSyncingService returns a Service over d whose marketplace sync is sync.
func newSyncingService(d Docker, sync func([]marketplace.Entry, func(marketplace.Entry) string) ([]marketplace.Result, error)) *Service {
	svc := New(d, nil)
	svc.syncMarketplaces = sync
	return svc
}

// approvedRC returns rc as approved config, as checkSettings would once the user approved it.
func approvedRC(rc *config.AgenticRC) approvedConfig {
	return approvedConfig{rc: rc}
}

// writeProxyLog writes lines as a proxy access log in a temp dir and returns its path.
func writeProxyLog(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "proxy_abc.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
	return path
}
