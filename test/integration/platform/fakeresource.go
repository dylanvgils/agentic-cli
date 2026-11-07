//go:build integration

package platform

import (
	"crypto/x509"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/network/certs"
	"github.com/stretchr/testify/assert"
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

// upstreamCAFile is the CA file in a fake upstream's cert dir.
const upstreamCAFile = "ca.pem"

// ListenPort is the TCP port a listening fake container accepts connections on.
const ListenPort = "8080"

// FakeResource builds a labelled docker container or network that looks like agentic created it.
type FakeResource struct {
	t        *testing.T
	kind     string
	name     string
	image    string
	labels   []string
	network  string
	aliases  []string
	env      []string
	mounts   []string
	user     string
	hardened bool
	started  bool
	upstream bool
	certDir  string
	port     string
	cmd      []string
}

// FakeContainer starts building a stopped container from the test tool image.
func FakeContainer(t *testing.T) *FakeResource {
	return &FakeResource{t: t, kind: "container", image: ToolImage, labels: fakeLabels()}
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

// FromImage builds the container from image instead of the test tool image.
func (f *FakeResource) FromImage(image string) *FakeResource {
	f.image = image
	return f
}

// WithEnv sets an env var in the container.
func (f *FakeResource) WithEnv(key, value string) *FakeResource {
	f.env = append(f.env, key+"="+value)
	return f
}

// WithMount bind-mounts src, which must live under RootDir, at dst.
func (f *FakeResource) WithMount(src, dst string) *FakeResource {
	f.mounts = append(f.mounts, src+":"+dst)
	return f
}

// RunningAs runs the container as user (uid:gid).
func (f *FakeResource) RunningAs(user string) *FakeResource {
	f.user = user
	return f
}

// Hardened runs the container read-only, without capabilities or privilege escalation, as agentic runs its sidecars.
func (f *FakeResource) Hardened() *FakeResource {
	f.hardened = true
	return f
}

// Started runs the container's own entrypoint, detached.
func (f *FakeResource) Started() *FakeResource {
	f.started = true
	return f
}

// ServingUpstream runs the fake upstream: HTTP on :80, HTTPS on :443 with a leaf for its aliases (see UpstreamCA).
func (f *FakeResource) ServingUpstream() *FakeResource {
	f.upstream = true
	f.port = "443"
	f.cmd = []string{"/fakeupstream"}
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

	if f.upstream {
		f.writeUpstreamCerts()
	}
	Docker(f.t, f.createArgs()...)
	f.t.Cleanup(func() { DockerOK(f.kind, "rm", "--force", f.name) })

	if f.port != "" {
		f.awaitListening()
	}
	return f
}

// createArgs returns the docker command creating the resource, running it when it has a command or was Started.
func (f *FakeResource) createArgs() []string {
	args := []string{f.kind, "create"}
	if f.cmd != nil || f.started {
		args = []string{f.kind, "run", "--detach"}
	}
	if f.cmd != nil {
		args = append(args, "--entrypoint", "")
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
	if f.kind != "container" {
		return append(args, f.name)
	}

	for _, e := range f.env {
		args = append(args, "--env", e)
	}
	for _, m := range f.mounts {
		args = append(args, "--volume", m)
	}
	if f.user != "" {
		args = append(args, "--user", f.user)
	}
	if f.hardened {
		args = append(args, "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true")
	}
	args = append(args, "--name", f.name, f.image)
	return append(args, f.cmd...)
}

// Exists reports whether the created resource is still there.
func (f *FakeResource) Exists() bool {
	return DockerOK(f.kind, "inspect", f.name)
}

// Name returns the created resource's name.
func (f *FakeResource) Name() string {
	return f.name
}

// UpstreamCA returns the host path of the CA that signed a ServingUpstream container's leaf, to mount into clients.
func (f *FakeResource) UpstreamCA() string {
	return filepath.Join(f.certDir, upstreamCAFile)
}

// IP returns the container's address on its network.
func (f *FakeResource) IP() string {
	f.t.Helper()
	return Docker(f.t, "inspect", "--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", f.name)
}

// writeUpstreamCerts writes a fresh CA and a leaf for the aliases, mounted with the fake upstream binary.
func (f *FakeResource) writeUpstreamCerts() {
	f.t.Helper()
	require.NotEmpty(f.t, f.aliases, "fake upstream needs an alias to issue its cert for")

	ca, err := certs.NewCA("agentic-itest upstream CA")
	require.NoError(f.t, err)
	// Its own CN, or OpenSSL takes a leaf named like its issuer for self-signed
	leaf, err := ca.IssueLeaf(f.aliases[0], x509.ExtKeyUsageServerAuth, f.aliases)
	require.NoError(f.t, err)
	key, err := leaf.KeyPEM()
	require.NoError(f.t, err)

	// Readable by any container user
	f.certDir = TempDirIn(f.t, rootDir)
	require.NoError(f.t, os.Chmod(f.certDir, 0o755))
	for name, data := range map[string][]byte{
		upstreamCAFile:     ca.CertPEM(),
		"upstream.pem":     leaf.CertPEM(),
		"upstream-key.pem": key,
	} {
		require.NoError(f.t, os.WriteFile(filepath.Join(f.certDir, name), data, 0o644))
	}
	f.mounts = append(f.mounts, fakeUpstreamBin+":/fakeupstream:ro", f.certDir+":/certs:ro")
}

// awaitListening waits until the container accepts connections on its port.
func (f *FakeResource) awaitListening() {
	f.t.Helper()
	listening := assert.Eventually(f.t, func() bool {
		return DockerOK("exec", f.name, "nc", "-z", "127.0.0.1", f.port)
	}, 10*time.Second, 100*time.Millisecond, "fake container %s never started listening", f.name)
	if !listening {
		logs, _ := exec.Command("docker", "logs", f.name).CombinedOutput()
		f.t.Fatalf("fake container %s logs:\n%s", f.name, logs)
	}
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
	out, err := dockerRun(append([]string{kind}, listArgs...)...)
	if err != nil {
		return fmt.Errorf("list fake %ss: %w", kind, err)
	}

	names := strings.Fields(out)
	if len(names) == 0 {
		return nil
	}
	if _, err := dockerRun(append([]string{kind, "rm", "--force"}, names...)...); err != nil {
		return fmt.Errorf("remove fake %ss: %w", kind, err)
	}

	for _, name := range names {
		fmt.Printf("   removed %s %s\n", kind, name)
	}
	return nil
}
