//go:build integration

package platform

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/runtime/microsandbox"
)

const (
	// dockerInfoFormat summarizes the daemon, incl. whether it can enforce resource limits
	dockerInfoFormat = "server {{.ServerVersion}}, cgroup driver {{.CgroupDriver}}, cgroup v{{.CgroupVersion}}"

	// imagesBuiltEnv tells Main buildimages already built the package's images, as make test-integration does.
	imagesBuiltEnv = "AGENTIC_ITEST_IMAGES_BUILT"

	// microsandboxEnv makes a missing msb fail instead of skipping its runs, as make test-integration MSB=1 does.
	microsandboxEnv = "AGENTIC_ITEST_MICROSANDBOX"

	// progressInterval is how often runningSteps.report prints the steps still running.
	progressInterval = 30 * time.Second
)

// ImageSet is the images a package needs built.
type ImageSet string

const (
	// BaseImages are the proxy and test tool images.
	BaseImages ImageSet = "base"

	// AllImages adds every tool's image and the test tool's with every extra layer, for the smoke tests.
	AllImages ImageSet = "all"
)

// smokePackages are the packages whose TestMain is SmokeMain or MicrosandboxMain, which need AllImages.
var smokePackages = []string{"images", "microsandbox"}

// step is one setup phase, timed in the output.
type step struct {
	name string
	run  func() error
}

// runningSteps tracks the names of the parallel steps that haven't finished yet.
type runningSteps struct {
	mu    sync.Mutex
	names map[string]bool
}

// newRunningSteps returns steps' names, all running.
func newRunningSteps(steps []step) *runningSteps {
	names := make(map[string]bool, len(steps))
	for _, s := range steps {
		names[s.name] = true
	}
	return &runningSteps{names: names}
}

// done marks the named step finished.
func (r *runningSteps) done(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.names, name)
}

// report prints the steps still running every progressInterval, until the returned stop is called.
func (r *runningSteps) report() (stop func()) {
	start := time.Now()
	ticker := time.NewTicker(progressInterval)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-quit:
				return
			case <-ticker.C:
				r.mu.Lock()
				names := slices.Sorted(maps.Keys(r.names))
				r.mu.Unlock()
				fmt.Printf("   still running after %s: %s\n", time.Since(start).Round(time.Second), strings.Join(names, ", "))
			}
		}
	}()

	return func() {
		ticker.Stop()
		close(quit)
	}
}

// ImagesFor returns the images the named packages need, every package's when none is named; mirrors their TestMain.
func ImagesFor(pkgs []string) ImageSet {
	if len(pkgs) == 0 {
		return AllImages
	}
	for _, pkg := range pkgs {
		if slices.Contains(smokePackages, pkg) {
			return AllImages
		}
	}
	return BaseImages
}

// BuildImages builds set's images in a throwaway env, so the packages' Main can skip them.
func BuildImages(set ImageSet) error {
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
	return runSteps(append(envSteps(), imageSteps(set)...))
}

// Main sets up the shared env with BaseImages, runs the package's tests and removes the env again.
func Main(m *testing.M) int {
	return runPackage(m, toolRunSteps(BaseImages))
}

// SmokeMain is Main for a package that runs every tool image: it builds AllImages.
func SmokeMain(m *testing.M) int {
	return runPackage(m, toolRunSteps(AllImages))
}

// RuntimesMain is Main for a package whose tests loop over Runtimes: microsandbox joins them when msb is available
// (required with microsandboxEnv), and the test images' msb copies are removed at the end.
func RuntimesMain(m *testing.M) int {
	ok, err := microsandboxAvailable()
	if err != nil {
		fmt.Println(err)
		return 1
	}
	if !ok {
		return Main(m)
	}

	runtimes = append(runtimes, microsandboxRuntime)
	return withMicrosandboxCleanup(Main(m))
}

// ProxyMain is Main for a package that runs the proxy image on its own: BaseImages, but no agentic run.
func ProxyMain(m *testing.M) int {
	steps := append(envSteps(), imageSteps(BaseImages)...)
	return runPackage(m, append(steps, step{"first proxy start", warmUpProxy}))
}

// MicrosandboxMain is SmokeMain for a package that runs tools in microsandbox: it skips without msb unless
// microsandboxEnv is set, and removes the test images' msb copies at the end.
func MicrosandboxMain(m *testing.M) int {
	ok, err := microsandboxAvailable()
	if err != nil || !ok {
		if err != nil {
			fmt.Println(err)
			return 1
		}
		fmt.Println("msb unavailable, skipping microsandbox integration tests")
		return 0
	}
	return withMicrosandboxCleanup(SmokeMain(m))
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
	fakeUpstreamBin = filepath.Join(rootDir, "fakeupstream")
	workDir = filepath.Join(rootDir, "work")
	agenticHome = filepath.Join(rootDir, "agentic-home")
	tmpDir = filepath.Join(rootDir, "tmp")
}

// runPackage sets up the env with steps, runs the package's tests and removes the env and this run's fakes again.
func runPackage(m *testing.M, steps []step) int {
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

// envSteps build the binaries and write the work dir every agentic call needs.
func envSteps() []step {
	return []step{
		{"building agentic binary", buildBinary},
		{"building fake upstream", buildFakeUpstream},
		{"writing work dir config", writeWorkDir},
	}
}

// toolRunSteps set up the env for agentic runs with set's images: approved settings and a first run.
func toolRunSteps(set ImageSet) []step {
	steps := append(envSteps(), imageSteps(set)...)
	return append(steps,
		step{"approving work dir settings", approveWorkDir},
		// With --proxy so it also creates agentic-net, the proxy's egress network the tests attach fakes to
		step{"first agentic run", func() error { return runAgentic("run", "--trust-dir", "--proxy", Tool, "--", "true") }},
	)
}

// imageSteps build set's images in parallel, none when imagesBuiltEnv says they're built.
func imageSteps(set ImageSet) []step {
	if os.Getenv(imagesBuiltEnv) != "" {
		return nil
	}

	builds := []step{
		proxyImageStep(),
		toolImageStep(Tool),
	}
	if set == AllImages {
		builds = append(builds, step{LayersNamespace + "-" + Tool, func() error {
			return runAgentic("build", "--namespace", LayersNamespace, "--base", strings.Join(tools.KnownExtras(), ","), Tool)
		}})
		for _, name := range tools.Names() {
			if name != Tool {
				builds = append(builds, toolImageStep(name))
			}
		}
	}

	return []step{
		{"migrating agentic home", migrateHome},
		{"building images", func() error { return runParallel(builds) }},
	}
}

// proxyImageStep builds the proxy image, so no image from older source runs the proxy tests.
func proxyImageStep() step {
	return step{"agentic-proxy", func() error { return runAgentic("proxy", "build") }}
}

// toolImageStep builds name's test image, cached after the first run.
func toolImageStep(name string) step {
	return step{Namespace + "-" + name, func() error { return runAgentic("build", name) }}
}

// migrateHome migrates the fresh AGENTIC_HOME once, or parallel builds would race through it.
func migrateHome() error {
	return runAgentic("migrate")
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

// runParallel runs steps concurrently, printing how long each took and, while they run, the ones still running, and
// returns their errors joined.
func runParallel(steps []step) error {
	running := newRunningSteps(steps)
	stop := running.report()
	defer stop()

	errs := make([]error, len(steps))
	var wg sync.WaitGroup
	for i, s := range steps {
		wg.Go(func() {
			start := time.Now()
			errs[i] = s.run()
			running.done(s.name)
			if errs[i] == nil {
				fmt.Printf("   %s done in %s\n", s.name, time.Since(start).Round(time.Millisecond))
			}
		})
	}

	wg.Wait()
	return errors.Join(errs...)
}

// buildBinary builds the agentic binary from this checkout.
func buildBinary() error {
	out, err := exec.Command("go", "build", "-o", agenticBin, "github.com/dylanvgils/agentic-cli/cmd/cli").CombinedOutput()
	if err != nil {
		return fmt.Errorf("build agentic: %w\n%s", err, out)
	}
	return nil
}

// buildFakeUpstream builds the static fake upstream FakeResource.ServingUpstream mounts, for the daemon's platform, not
// the host's (macOS).
func buildFakeUpstream() error {
	platform, err := dockerRun("version", "--format", "{{.Server.Os}} {{.Server.Arch}}")
	if err != nil {
		return fmt.Errorf("read the daemon platform: %w", err)
	}
	goos, goarch, _ := strings.Cut(platform, " ")

	cmd := exec.Command("go", "build", "-tags", "integration", "-o", fakeUpstreamBin, "github.com/dylanvgils/agentic-cli/test/integration/cmd/fakeupstream")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+goos, "GOARCH="+goarch)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("build fake upstream: %w\n%s", err, out)
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

// warmUpProxy starts the proxy image once, hardened, before any test: the first such start can stall on a fresh daemon
// (seen with a rootless sidecar), as Main's first agentic run also absorbs. The proxy exits at once without its address.
func warmUpProxy() error {
	_, _ = dockerRun("run", "--rm", "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true", tools.ProxyImage)
	return nil
}

// microsandboxAvailable reports whether msb is installed, failing when it isn't but microsandboxEnv requires it.
func microsandboxAvailable() (bool, error) {
	version, err := exec.Command("msb", "--version").CombinedOutput()
	if err != nil {
		if os.Getenv(microsandboxEnv) != "" {
			return false, fmt.Errorf("msb unavailable: %w\n%s", err, version)
		}
		return false, nil
	}

	fmt.Printf("=> msb: %s\n", strings.TrimSpace(string(version)))
	return true, nil
}

// withMicrosandboxCleanup removes the test images' msb copies and returns code.
func withMicrosandboxCleanup(code int) int {
	if err := runSteps([]step{{"removing the test images from msb", removeMicrosandboxImages}}); err != nil {
		fmt.Println(err)
	}
	return code
}

// removeMicrosandboxImages removes the msb copies of the test images, which agentic loads on the first msb run.
func removeMicrosandboxImages() error {
	out, err := exec.Command("msb", "images", "--quiet").Output()
	if err != nil {
		return fmt.Errorf("list msb images: %w", err)
	}

	var errs []error
	for ref := range strings.FieldsSeq(string(out)) {
		if repo, _ := microsandbox.SplitRef(ref); !strings.HasPrefix(repo, Namespace+"-") {
			continue
		}
		if out, err := exec.Command("msb", "rmi", ref).CombinedOutput(); err != nil {
			errs = append(errs, fmt.Errorf("msb rmi %s: %w\n%s", ref, err, out))
		}
	}
	return errors.Join(errs...)
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
