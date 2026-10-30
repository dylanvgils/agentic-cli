//go:build integration

// Package platform sets up the shared env the integration tests run the agentic binary in, against a real Docker daemon,
// and the builders they drive it with.
package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dylanvgils/agentic-cli/internal/config"
)

const (
	// Namespace keeps test images apart from the user's own.
	Namespace = "agentic-itest"

	// Tool is the tool every test runs.
	Tool = "claude"

	// ToolImage is the test tool's image.
	ToolImage = Namespace + "-" + Tool

	// Differ from the defaults, so a limit inherited from the host can't pass as applied
	PidsLimit = "321"
	Memory    = "300m"
	CPUs      = "1.5"
)

// rcContents isolates the work dir from the repo's and user's .agenticrc.toml; netcat is for the network tests.
var rcContents = fmt.Sprintf(`root = true
namespace = %q

[build]
apt_packages = ["netcat-openbsd"]

[run]
check_updates = false
pids_limit = %q
memory = %q
cpus = %q
`, Namespace, PidsLimit, Memory, CPUs)

var (
	// rootDir holds every host file the tests create, under the repo so a sidecar daemon can bind-mount them.
	rootDir string

	// agenticBin is the binary built from this checkout.
	agenticBin string

	// workDir is the cwd for every agentic call; mounted as the tool's /workspace.
	workDir string

	// agenticHome is the AGENTIC_HOME for every agentic call.
	agenticHome string

	// tmpDir is the TMPDIR for every agentic call, since it bind-mounts generated files from there.
	tmpDir string

	// runID labels this package run's fakes, so its cleanup spares the parallel packages' ones.
	runID string
)

// AgenticCmd returns an agentic command running in the shared work dir with the test AGENTIC_HOME and TMPDIR.
func AgenticCmd(args ...string) *exec.Cmd {
	return AgenticCmdIn(workDir, args...)
}

// AgenticCmdIn is AgenticCmd running in dir.
func AgenticCmdIn(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command(agenticBin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "AGENTIC_HOME="+agenticHome, "TMPDIR="+tmpDir)
	return cmd
}

// RootDir returns the dir holding every host file the tests create; put bind-mount sources under it.
func RootDir() string {
	return rootDir
}

// WorkDir returns the shared work dir, mounted as the tool's /workspace.
func WorkDir() string {
	return workDir
}

// approveSettings records the guarded settings in the .agenticrc.toml at rcPath as approved, as the settings prompt would.
func approveSettings(rcPath string) error {
	layers, err := config.FindLayers(filepath.Dir(rcPath))
	if err != nil {
		return err
	}
	cfg, err := config.LoadState(agenticHome)
	if err != nil {
		return err
	}

	for _, layer := range layers {
		if layer.Path == rcPath {
			return cfg.ApproveSettings(layer, agenticHome)
		}
	}
	return nil
}
