//go:build integration

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	// dockerInfoFormat summarizes the daemon, incl. whether it can enforce resource limits
	dockerInfoFormat = "server {{.ServerVersion}}, cgroup driver {{.CgroupDriver}}, cgroup v{{.CgroupVersion}}"

	// imagesBuiltEnv tells Main the images are already built, as make test-integration does once for every package.
	imagesBuiltEnv = "AGENTIC_ITEST_IMAGES_BUILT"
)

// step is one setup phase, timed in the output.
type step struct {
	name string
	run  func() error
}

// BuildImages builds the test tool and proxy images in a throwaway env, so the packages' Main can skip them.
func BuildImages() error {
	skip, err := checkDocker()
	if skip || err != nil {
		return err
	}

	root, err := newRootDir()
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(root) }()

	setPaths(root)
	return runSteps(append(envSteps(), imageSteps()...))
}

// Main sets up the shared env, runs the package's tests and removes the env again; call it from each TestMain.
func Main(m *testing.M) int {
	skip, err := checkDocker()
	if err != nil {
		fmt.Println(err)
		return 1
	}
	if skip {
		return 0
	}

	root, err := newRootDir()
	if err != nil {
		fmt.Println(err)
		return 1
	}
	defer func() { _ = os.RemoveAll(root) }()

	setPaths(root)
	steps := envSteps()
	if os.Getenv(imagesBuiltEnv) == "" {
		steps = append(steps, imageSteps()...)
	}
	steps = append(steps,
		step{"approving work dir settings", approveWorkDir},
		// With --proxy so it also creates agentic-net, the proxy's egress network the tests attach fakes to
		step{"first agentic run", func() error { return runAgentic("run", "--trust-dir", "--proxy", Tool, "--", "true") }},
	)

	if err := runSteps(steps); err != nil {
		fmt.Println(err)
		return 1
	}

	code := m.Run()
	if err := runSteps([]step{{"removing this run's fakes", removeRunFakes}}); err != nil {
		fmt.Println(err)
	}
	return code
}

// checkDocker prints the daemon summary; without a daemon it skips, except on CI where skipping would hide that nothing ran.
func checkDocker() (skip bool, err error) {
	info, err := dockerRun("info", "--format", dockerInfoFormat)
	if err != nil {
		if os.Getenv("CI") != "" {
			return false, fmt.Errorf("docker unavailable: %w", err)
		}
		fmt.Printf("docker unavailable, skipping integration tests: %v\n", err)
		return true, nil
	}

	fmt.Printf("=> docker: %s\n", info)
	return false, nil
}

// setPaths points the env at root.
func setPaths(root string) {
	rootDir = root
	runID = filepath.Base(root)
	agenticBin = filepath.Join(rootDir, "agentic")
	workDir = filepath.Join(rootDir, "work")
	agenticHome = filepath.Join(rootDir, "agentic-home")
	tmpDir = filepath.Join(rootDir, "tmp")
}

// envSteps build the binary and write the work dir every agentic call needs.
func envSteps() []step {
	return []step{
		{"building agentic binary", buildBinary},
		{"writing work dir config", writeWorkDir},
	}
}

// imageSteps build the test tool and proxy images.
func imageSteps() []step {
	return []step{
		// Cached after the first run
		{"building " + ToolImage + " image", func() error { return runAgentic("build", Tool) }},
		// Otherwise a proxy image from older source would run the proxy tests
		{"building agentic-proxy image", func() error { return runAgentic("proxy", "build") }},
	}
}

// runSteps runs steps in order, printing how long each took.
func runSteps(steps []step) error {
	for _, s := range steps {
		fmt.Printf("=> %s...\n", s.name)
		start := time.Now()
		if err := s.run(); err != nil {
			return err
		}
		fmt.Printf("   done in %s\n", time.Since(start).Round(time.Millisecond))
	}

	return nil
}

// buildBinary builds the agentic binary from this checkout.
func buildBinary() error {
	out, err := exec.Command("go", "build", "-o", agenticBin, "github.com/dylanvgils/agentic-cli/cmd/cli").CombinedOutput()
	if err != nil {
		return fmt.Errorf("build agentic: %w\n%s", err, out)
	}
	return nil
}

// writeWorkDir creates the env's dirs and writes the work dir's .agenticrc.toml.
func writeWorkDir() error {
	// AGENTIC_HOME too, since with prebuilt images no build creates it before the approval
	for _, dir := range []string{workDir, tmpDir, agenticHome} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(workDir, ".agenticrc.toml"), []byte(rcContents), 0o644)
}

// approveWorkDir approves the work dir's .agenticrc.toml settings.
func approveWorkDir() error {
	if err := approveSettings(filepath.Join(workDir, ".agenticrc.toml")); err != nil {
		return fmt.Errorf("approve work dir settings: %w", err)
	}
	return nil
}

// runAgentic runs agentic in the shared work dir, returning its output with the error.
func runAgentic(args ...string) error {
	if out, err := AgenticCmd(args...).CombinedOutput(); err != nil {
		return fmt.Errorf("agentic %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return nil
}

// newRootDir creates the env's root dir under test/integration in this checkout, the same parent for every package's
// test binary, since a sidecar daemon only sees /workspace bind-mount sources.
func newRootDir() (string, error) {
	gomod, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		return "", fmt.Errorf("find module root: %w", err)
	}
	parent := filepath.Join(filepath.Dir(strings.TrimSpace(string(gomod))), "test", "integration")
	return os.MkdirTemp(parent, ".home-")
}
