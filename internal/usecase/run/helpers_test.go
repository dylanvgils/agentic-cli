package run

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/stretchr/testify/require"
)

// fakeDocker implements Docker; a nil field succeeds with a zero value.
type fakeDocker struct {
	inspectImage         func(string) (*docker.ImageInfo, error)
	resolveContainerHome func(string) string
	ensureNamedVolumes   func(volumes []string, toolHome, containerHome, chownImage string) error
	ensureNetwork        func() error
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

// stubLoggingErr redirects logging.Err to a buffer for the duration of the test and returns it.
func stubLoggingErr(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := logging.Err
	logging.Err = logging.New(&buf)
	t.Cleanup(func() { logging.Err = orig })
	return &buf
}
