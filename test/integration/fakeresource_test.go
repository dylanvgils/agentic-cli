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

// listenPort is the TCP port a listening fake container accepts connections on.
const listenPort = "8080"

// fakeResource builds a labelled docker container or network that looks like agentic created it.
type fakeResource struct {
	t       *testing.T
	kind    string
	name    string
	labels  []string
	network string
	aliases []string
	port    string
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

// withAlias gives the container a DNS alias on its network.
func (f *fakeResource) withAlias(alias string) *fakeResource {
	f.aliases = append(f.aliases, alias)
	return f
}

// listening makes the container run a TCP server on listenPort.
func (f *fakeResource) listening() *fakeResource {
	return f.listeningOn(listenPort)
}

// listeningOn makes the container run a TCP server on port.
func (f *fakeResource) listeningOn(port string) *fakeResource {
	f.port = port
	f.cmd = []string{"nc", "-lk", port}
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
	for _, a := range f.aliases {
		args = append(args, "--network-alias", a)
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

// ip returns the container's address on its network.
func (f *fakeResource) ip() string {
	f.t.Helper()
	out, err := exec.Command("docker", "inspect", "--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", f.name).Output()
	require.NoError(f.t, err)
	return strings.TrimSpace(string(out))
}

// awaitListening waits until the container accepts connections on its port.
func (f *fakeResource) awaitListening() {
	f.t.Helper()
	require.Eventually(f.t, func() bool {
		return exec.Command("docker", "exec", f.name, "nc", "-z", "127.0.0.1", f.port).Run() == nil
	}, 10*time.Second, 100*time.Millisecond, "fake container %s never started listening", f.name)
}
