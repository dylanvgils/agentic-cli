//go:build integration

// Package integration runs the agentic binary against a real Docker daemon.
package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const (
	// testNamespace keeps test images apart from the user's own.
	testNamespace = "agentic-itest"

	// testTool is the tool image every test runs.
	testTool = "claude"

	// Differ from the defaults, so a limit inherited from the host can't pass as applied
	testPidsLimit = "321"
	testMemory    = "300m"
)

// rcContents isolates the work dir from the repo's and user's .agenticrc.toml.
var rcContents = fmt.Sprintf("root = true\nnamespace = %q\n\n[run]\ncheck_updates = false\npids_limit = %q\nmemory = %q\n", testNamespace, testPidsLimit, testMemory)

var (
	// agenticBin is the binary built from this checkout.
	agenticBin string

	// workDir is the cwd for every agentic call; mounted as the tool's /workspace.
	workDir string

	// agenticHome is the AGENTIC_HOME for every agentic call.
	agenticHome string

	// tmpDir is the TMPDIR for every agentic call, since it bind-mounts generated files from there.
	tmpDir string
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

// run sets up the shared env, runs the tests and removes the env again.
func run(m *testing.M) int {
	if out, err := exec.Command("docker", "version").CombinedOutput(); err != nil {
		fmt.Printf("skipping integration tests: docker unavailable: %v\n%s", err, out)
		return 0
	}

	// Under the repo, since a sidecar daemon only sees /workspace bind-mount sources
	root, err := os.MkdirTemp(".", ".home-")
	if err != nil {
		fmt.Println(err)
		return 1
	}
	defer func() { _ = os.RemoveAll(root) }()

	if err := setup(root); err != nil {
		fmt.Println(err)
		return 1
	}
	return m.Run()
}

// setup builds the binary, writes the work dir config and builds the test tool image.
func setup(root string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}

	agenticBin = filepath.Join(root, "agentic")
	workDir = filepath.Join(root, "work")
	agenticHome = filepath.Join(root, "agentic-home")
	tmpDir = filepath.Join(root, "tmp")

	build := exec.Command("go", "build", "-o", agenticBin, "../../cmd/cli")
	if out, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("build agentic: %w\n%s", err, out)
	}

	for _, dir := range []string{workDir, tmpDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(workDir, ".agenticrc.toml"), []byte(rcContents), 0o644); err != nil {
		return err
	}

	// Cached after the first run
	if out, err := agenticCmd("build", testTool).CombinedOutput(); err != nil {
		return fmt.Errorf("build %s image: %w\n%s", testTool, err, out)
	}
	return nil
}
