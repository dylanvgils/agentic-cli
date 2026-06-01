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
	"github.com/spf13/cobra"
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

// captureRunContainer replaces runContainer and ensureNamedVolumes with stubs
// that record the RunSpec and tool args. Returns a getter for the captured values.
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
	ensureNamedVolumes = func(volumes []string, toolHome, containerHome string) error {
		return nil
	}

	t.Cleanup(func() {
		runContainer = origRun
		ensureNamedVolumes = origEnsure
	})

	return func() (docker.RunSpec, []string) { return capturedSpec, capturedArgs }
}

// newFlagCmd creates a cobra command with the given string flags registered.
func newFlagCmd(t *testing.T, flags ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	for _, f := range flags {
		cmd.Flags().String(f, "", "")
	}
	return cmd
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

func stubBuildTool(t *testing.T, fn func(tool, image string, opts tools.BuildOptions) error) {
	t.Helper()
	orig := buildTool
	buildTool = fn
	t.Cleanup(func() { buildTool = orig })
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

func stubListAllAgenticImages(t *testing.T, fn func() ([]*docker.ImageInfo, error)) {
	t.Helper()
	orig := listAllAgenticImages
	listAllAgenticImages = fn
	t.Cleanup(func() { listAllAgenticImages = orig })
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

func stubPruneImages(t *testing.T, fn func() (string, error)) {
	t.Helper()
	orig := pruneImages
	pruneImages = fn
	t.Cleanup(func() { pruneImages = orig })
}

func stubRemoveVolume(t *testing.T, fn func(string) error) {
	t.Helper()
	orig := removeVolume
	removeVolume = fn
	t.Cleanup(func() { removeVolume = orig })
}

func stubUpdateTool(t *testing.T, fn func(tool, image string, opts tools.BuildOptions) error) {
	t.Helper()
	orig := updateTool
	updateTool = fn
	t.Cleanup(func() { updateTool = orig })
}

func stubCurrentGOOS(t *testing.T, goos string) {
	t.Helper()
	orig := currentGOOS
	currentGOOS = goos
	t.Cleanup(func() { currentGOOS = orig })
}

func stubVolumeStdin(t *testing.T, input string) {
	t.Helper()
	orig := volumesStdin
	volumesStdin = strings.NewReader(input)
	t.Cleanup(func() { volumesStdin = orig })
}
