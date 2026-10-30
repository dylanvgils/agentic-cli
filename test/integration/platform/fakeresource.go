//go:build integration

package platform

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Mirrors internal/runtime/docker labels; duplicated so the tests stay black-box
const (
	labelProject    = "project=agentic-cli"
	labelOwner      = "agentic.owner"
	labelStarted    = "agentic.started"
	labelTimeFormat = "2006-01-02T15:04:05Z"
)

// labelRun marks the fakes of one package run, so the end-of-run cleanup catches the ones a test's cleanup missed.
const labelRun = "agentic.itest.run"

// ListenPort is the TCP port a listening fake container accepts connections on.
const ListenPort = "8080"

// FakeResource builds a labelled docker container or network that looks like agentic created it.
type FakeResource struct {
	t       *testing.T
	kind    string
	name    string
	labels  []string
	network string
	aliases []string
	port    string
	cmd     []string
}

// FakeContainer starts building a stopped container from the test tool image.
func FakeContainer(t *testing.T) *FakeResource {
	return &FakeResource{t: t, kind: "container", labels: fakeLabels()}
}

// FakeNetwork starts building a network.
func FakeNetwork(t *testing.T) *FakeResource {
	return &FakeResource{t: t, kind: "network", labels: fakeLabels()}
}

// OwnedBy labels the resource as a sidecar (network) of the owner container.
func (f *FakeResource) OwnedBy(owner string) *FakeResource {
	f.labels = append(f.labels, labelOwner+"="+owner)
	return f
}

// StartedAt labels when the resource was created.
func (f *FakeResource) StartedAt(started time.Time) *FakeResource {
	f.labels = append(f.labels, labelStarted+"="+started.UTC().Format(labelTimeFormat))
	return f
}

// OnNetwork attaches the container to network.
func (f *FakeResource) OnNetwork(network string) *FakeResource {
	f.network = network
	return f
}

// WithAlias gives the container a DNS alias on its network.
func (f *FakeResource) WithAlias(alias string) *FakeResource {
	f.aliases = append(f.aliases, alias)
	return f
}

// Listening makes the container run a TCP server on ListenPort.
func (f *FakeResource) Listening() *FakeResource {
	return f.ListeningOn(ListenPort)
}

// ListeningOn makes the container run a TCP server on port.
func (f *FakeResource) ListeningOn(port string) *FakeResource {
	f.port = port
	f.cmd = []string{"nc", "-lk", port}
	return f
}

// Create creates the resource under a unique name and removes it on cleanup; a listening container is started and awaited.
func (f *FakeResource) Create() *FakeResource {
	f.t.Helper()
	f.name = UniqueName(f.t)

	args := []string{f.kind, "create"}
	if f.cmd != nil {
		args = []string{f.kind, "run", "--detach", "--entrypoint", ""}
	}
	for _, l := range f.labels {
		args = append(args, "--label", l)
	}
	if f.network != "" {
		args = append(args, "--network", f.network)
	}
	for _, a := range f.aliases {
		args = append(args, "--network-alias", a)
	}

	if f.kind == "container" {
		args = append(args, "--name", f.name, ToolImage)
		args = append(args, f.cmd...)
	} else {
		args = append(args, f.name)
	}

	Docker(f.t, args...)
	f.t.Cleanup(func() { _ = exec.Command("docker", f.kind, "rm", "--force", f.name).Run() })

	if f.cmd != nil {
		f.awaitListening()
	}
	return f
}

// Exists reports whether the created resource is still there.
func (f *FakeResource) Exists() bool {
	return exec.Command("docker", f.kind, "inspect", f.name).Run() == nil
}

// Name returns the created resource's name.
func (f *FakeResource) Name() string {
	return f.name
}

// IP returns the container's address on its network.
func (f *FakeResource) IP() string {
	f.t.Helper()
	out, err := exec.Command("docker", "inspect", "--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", f.name).Output()
	require.NoError(f.t, err)
	return strings.TrimSpace(string(out))
}

// awaitListening waits until the container accepts connections on its port.
func (f *FakeResource) awaitListening() {
	f.t.Helper()
	require.Eventually(f.t, func() bool {
		return exec.Command("docker", "exec", f.name, "nc", "-z", "127.0.0.1", f.port).Run() == nil
	}, 10*time.Second, 100*time.Millisecond, "fake container %s never started listening", f.name)
}

// RemoveAllFakes force-removes the fakes of every package run, also of one whose test binary died.
func RemoveAllFakes() error {
	return runSteps([]step{
		{"removing leftover fakes of every run", func() error { return removeFakes("label=" + labelRun) }},
	})
}

// fakeLabels returns the labels every fake starts with.
func fakeLabels() []string {
	return []string{
		labelProject,
		labelRun + "=" + runID,
	}
}

// removeRunFakes force-removes the fakes of this package run, sparing the parallel packages' ones.
func removeRunFakes() error {
	return removeFakes("label=" + labelRun + "=" + runID)
}

// removeFakes force-removes the fake containers, then networks, matching the docker filter.
func removeFakes(filter string) error {
	// Containers first, so no fake is still attached to a network
	if err := removeFakesOf("container", "ls", "--all", "--format", "{{.Names}}", "--filter", filter); err != nil {
		return err
	}
	return removeFakesOf("network", "ls", "--format", "{{.Name}}", "--filter", filter)
}

// removeFakesOf force-removes the resources of kind that docker <kind> ls with listArgs names, printing each.
func removeFakesOf(kind string, listArgs ...string) error {
	out, err := exec.Command("docker", append([]string{kind}, listArgs...)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("list fake %ss: %w\n%s", kind, err, out)
	}

	names := strings.Fields(string(out))
	if len(names) == 0 {
		return nil
	}
	if out, err := exec.Command("docker", append([]string{kind, "rm", "--force"}, names...)...).CombinedOutput(); err != nil {
		return fmt.Errorf("remove fake %ss: %w\n%s", kind, err, out)
	}

	for _, name := range names {
		fmt.Printf("   removed %s %s\n", kind, name)
	}
	return nil
}
