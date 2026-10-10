//go:build integration

// Package integration runs the agentic binary against a real Docker daemon.
package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const (
	// testNamespace keeps test images apart from the user's own.
	testNamespace = "agentic-itest"

	// testTool is the tool image every test runs.
	testTool = "claude"

	// Differ from the defaults, so a limit inherited from the host can't pass as applied
	testPidsLimit = "321"
	testMemory    = "300m"
	testCPUs      = "1.5"

	// dockerInfoFormat summarizes the daemon, incl. whether it can enforce resource limits
	dockerInfoFormat = "server {{.ServerVersion}}, cgroup driver {{.CgroupDriver}}, cgroup v{{.CgroupVersion}}"
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
`, testNamespace, testPidsLimit, testMemory, testCPUs)

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
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

// run sets up the shared env, runs the tests and removes the env again.
func run(m *testing.M) int {
	info, err := exec.Command("docker", "info", "--format", dockerInfoFormat).CombinedOutput()
	if err != nil {
		fmt.Printf("docker unavailable: %v\n%s", err, info)
		// Skipping on CI would hide that nothing ran
		if os.Getenv("CI") != "" {
			return 1
		}
		fmt.Println("skipping integration tests")
		return 0
	}
	fmt.Printf("=> docker: %s", info)

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

// setup builds the binary, writes the work dir config, builds the test tool and proxy images and runs the tool once.
func setup(root string) error {
	var err error
	rootDir, err = filepath.Abs(root)
	if err != nil {
		return err
	}

	agenticBin = filepath.Join(rootDir, "agentic")
	workDir = filepath.Join(rootDir, "work")
	agenticHome = filepath.Join(rootDir, "agentic-home")
	tmpDir = filepath.Join(rootDir, "tmp")

	done := logStep("building agentic binary")
	build := exec.Command("go", "build", "-o", agenticBin, "../../cmd/cli")
	if out, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("build agentic: %w\n%s", err, out)
	}
	done()

	for _, dir := range []string{workDir, tmpDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(workDir, ".agenticrc.toml"), []byte(rcContents), 0o644); err != nil {
		return err
	}

	// Cached after the first run
	done = logStep("building " + testNamespace + "-" + testTool + " image")
	if out, err := agenticCmd("build", testTool).CombinedOutput(); err != nil {
		return fmt.Errorf("build %s image: %w\n%s", testTool, err, out)
	}
	done()

	// Otherwise a proxy image from older source would run the proxy tests
	done = logStep("building agentic-proxy image")
	if out, err := agenticCmd("proxy", "build").CombinedOutput(); err != nil {
		return fmt.Errorf("build proxy image: %w\n%s", err, out)
	}
	done()

	if err := approveSettings(filepath.Join(workDir, ".agenticrc.toml")); err != nil {
		return fmt.Errorf("approve work dir settings: %w", err)
	}

	// Also creates agentic-net, which the network tests attach fakes to
	done = logStep("first agentic run")
	if out, err := agenticCmd("run", "--trust-dir", testTool, "--", "true").CombinedOutput(); err != nil {
		return fmt.Errorf("first run: %w\n%s", err, out)
	}
	done()
	return nil
}

// logStep prints msg and returns a func that prints how long the step took.
func logStep(msg string) func() {
	fmt.Printf("=> %s...\n", msg)
	start := time.Now()
	return func() { fmt.Printf("   done in %s\n", time.Since(start).Round(time.Millisecond)) }
}
