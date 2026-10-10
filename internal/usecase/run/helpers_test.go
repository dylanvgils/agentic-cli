package run

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/toolupdate"
	"github.com/stretchr/testify/require"
)

// fakeDocker implements Docker; a nil field succeeds with a zero value.
type fakeDocker struct {
	inspectImage         func(string) (*docker.ImageInfo, error)
	resolveContainerHome func(string) string
	ensureNamedVolumes   func(volumes []string, toolHome, containerHome, chownImage string) error
	ensureNetwork        func() error
	listAllImages        func(...docker.ImageFilter) ([]*docker.ImageInfo, error)
	buildProxyImage      func(image, version, sourceDir string, opts tools.BuildOptions) error
	buildDindImage       func(image string, opts tools.BuildOptions) error
	buildTool            func(tool, image string, opts tools.BuildOptions) error
	restampImage         func(image string, info docker.ImageInfo)
	pruneDangling        func() error
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

func (f *fakeDocker) EnsureNamedVolumes(volumes []string, toolHome, containerHome, chownImage string) error {
	if f.ensureNamedVolumes == nil {
		return nil
	}
	return f.ensureNamedVolumes(volumes, toolHome, containerHome, chownImage)
}

func (f *fakeDocker) EnsureNetwork() error {
	if f.ensureNetwork == nil {
		return nil
	}
	return f.ensureNetwork()
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

func (f *fakeDocker) PruneDangling() error {
	if f.pruneDangling == nil {
		return nil
	}
	return f.pruneDangling()
}

// fakePrompter implements Prompter, recording what it was asked; a nil field approves, except tool updates, which it declines.
type fakePrompter struct {
	trustDir           func(string) error
	approveCredentials func(config.RCLayer) error
	offerToolUpdate    func(tool, installed, latest string) bool

	trustAsked       []string
	credentialsAsked []string
	updatesOffered   []string
}

func (f *fakePrompter) TrustDir(dir string) error {
	f.trustAsked = append(f.trustAsked, dir)
	if f.trustDir == nil {
		return nil
	}
	return f.trustDir(dir)
}

func (f *fakePrompter) ApproveCredentials(layer config.RCLayer) error {
	f.credentialsAsked = append(f.credentialsAsked, layer.Path)
	if f.approveCredentials == nil {
		return nil
	}
	return f.approveCredentials(layer)
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

func stubSyncMarketplaces(t *testing.T, fn func([]marketplace.Entry, func(marketplace.Entry) string) ([]marketplace.Result, error)) {
	t.Helper()
	orig := SyncMarketplaces
	SyncMarketplaces = fn
	t.Cleanup(func() { SyncMarketplaces = orig })
}

func stubRecordMarketplaceUsage(t *testing.T, fn func(baseDir string, results []marketplace.Result, projectDir string) error) {
	t.Helper()
	orig := RecordMarketplaceUsage
	RecordMarketplaceUsage = fn
	t.Cleanup(func() { RecordMarketplaceUsage = orig })
}

// credentialLayer returns a config layer declaring one generic credential reading secret.
func credentialLayer(t *testing.T, secret string) config.RCLayer {
	t.Helper()
	rc := &config.AgenticRC{}
	rc.Run.Proxy.Credentials = []config.RCCredential{{Hosts: []string{"api.example.test"}, Header: "X-Api-Key", Secret: secret}}
	return config.RCLayer{Path: filepath.Join(t.TempDir(), ".agenticrc.toml"), RC: rc}
}

// approveCredentials records layer's credentials as approved in toolHome's agentic.json.
func approveCredentials(t *testing.T, layer config.RCLayer, toolHome string) {
	t.Helper()
	cfg, err := config.LoadConfig(toolHome)
	require.NoError(t, err)
	require.NoError(t, cfg.ApproveCredentials(layer.Path, config.CredentialsHash(layer.RC.Run.Proxy.Credentials), toolHome))
}

// stubCaseInsensitivePaths sets whether path checks ignore case for the duration of the test.
func stubCaseInsensitivePaths(t *testing.T, val bool) {
	t.Helper()
	orig := caseInsensitivePaths
	caseInsensitivePaths = val
	t.Cleanup(func() { caseInsensitivePaths = orig })
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

// stubLatestToolVersion makes the tool update check see latest as newer for the duration of the test.
func stubLatestToolVersion(t *testing.T, latest string) {
	t.Helper()
	orig := toolupdate.LatestToolVersion
	toolupdate.LatestToolVersion = func(string, string) (string, bool, bool) { return latest, true, true }
	t.Cleanup(func() { toolupdate.LatestToolVersion = orig })
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

	real := filepath.Join(dir, "real")
	require.NoError(t, os.MkdirAll(filepath.Join(real, "sub"), 0o700))
	require.NoError(t, os.Symlink(real, filepath.Join(dir, "link")))
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "escape")))
	return symlinkWorkspace{dir: dir, real: real, outside: outside}
}
