//go:build integration

package integration

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Mirrors internal/docker labels; duplicated so the tests stay black-box
const (
	labelProject    = "project=agentic-cli"
	labelOwner      = "agentic.owner"
	labelStarted    = "agentic.started"
	labelTimeFormat = "2006-01-02T15:04:05Z"
)

const (
	// listenPort is the TCP port a listening fake container accepts connections on.
	listenPort = "8080"

	// listenScript is a perl TCP server (the only listener the tool image has) that logs once it's ready.
	listenScript = `$| = 1; my $s = IO::Socket::INET->new(LocalPort => ` + listenPort + `, Listen => 5, ReuseAddr => 1) or die $!; print "listening\n"; while (my $c = $s->accept) { close $c }`
)

// fakeResource builds a labelled docker container or network that looks like agentic created it.
type fakeResource struct {
	t       *testing.T
	kind    string
	name    string
	labels  []string
	network string
	cmd     []string
}

// fakeContainer starts building a stopped container from the test tool image.
func fakeContainer(t *testing.T) *fakeResource {
	return &fakeResource{t: t, kind: "container", labels: []string{labelProject}}
}

// fakeNetwork starts building a network.
func fakeNetwork(t *testing.T) *fakeResource {
	return &fakeResource{t: t, kind: "network", labels: []string{labelProject}}
}

// ownedBy labels the resource as a sidecar (network) of the owner container.
func (f *fakeResource) ownedBy(owner string) *fakeResource {
	f.labels = append(f.labels, labelOwner+"="+owner)
	return f
}

// startedAt labels when the resource was created.
func (f *fakeResource) startedAt(started time.Time) *fakeResource {
	f.labels = append(f.labels, labelStarted+"="+started.UTC().Format(labelTimeFormat))
	return f
}

// onNetwork attaches the container to network.
func (f *fakeResource) onNetwork(network string) *fakeResource {
	f.network = network
	return f
}

// listening makes the container run a TCP server on listenPort.
func (f *fakeResource) listening() *fakeResource {
	f.cmd = []string{"perl", "-MIO::Socket::INET", "-e", listenScript}
	return f
}

// create creates the resource under a unique name and removes it on cleanup; a listening container is started and awaited.
func (f *fakeResource) create() *fakeResource {
	f.t.Helper()
	f.name = uniqueName(f.t)

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

	if f.kind == "container" {
		args = append(args, "--name", f.name, testNamespace+"-"+testTool)
		args = append(args, f.cmd...)
	} else {
		args = append(args, f.name)
	}

	docker(f.t, args...)
	f.t.Cleanup(func() { _ = exec.Command("docker", f.kind, "rm", "--force", f.name).Run() })

	if f.cmd != nil {
		f.awaitListening()
	}
	return f
}

// exists reports whether the created resource is still there.
func (f *fakeResource) exists() bool {
	return exec.Command("docker", f.kind, "inspect", f.name).Run() == nil
}

// addr returns the container's ip:listenPort on its network.
func (f *fakeResource) addr() string {
	f.t.Helper()
	out, err := exec.Command("docker", "inspect", "--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", f.name).Output()
	require.NoError(f.t, err)
	return strings.TrimSpace(string(out)) + ":" + listenPort
}

// awaitListening waits until the container's server logs that it's ready.
func (f *fakeResource) awaitListening() {
	f.t.Helper()
	require.Eventually(f.t, func() bool {
		out, _ := exec.Command("docker", "logs", f.name).CombinedOutput()
		return strings.Contains(string(out), "listening")
	}, 10*time.Second, 100*time.Millisecond, "fake container %s never started listening", f.name)
}
