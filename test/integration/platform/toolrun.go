//go:build integration

package platform

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waitScript signals the tool is up by writing its hostname, the container ID, to .ready, then keeps it running until
// .done exists.
const waitScript = "hostname > .ready.tmp && mv .ready.tmp .ready; while [ ! -f .done ]; do sleep 0.1; done"

// credentialRule is a [[run.proxy.credentials]] entry.
type credentialRule struct {
	Hosts  []string `toml:"hosts"`
	Header string   `toml:"header"`
	Env    []string `toml:"env"`
	Secret string   `toml:"secret"`
}

// pathRule is a [[run.proxy.allowed_paths]] entry.
type pathRule struct {
	Host  string   `toml:"host"`
	Paths []string `toml:"paths"`
}

// proxyTable is the [run.proxy] table a run adds to rcContents.
type proxyTable struct {
	AllowedHosts []string         `toml:"allowed_hosts,omitempty"`
	Credentials  []credentialRule `toml:"credentials,omitempty"`
	AllowedPaths []pathRule       `toml:"allowed_paths,omitempty"`
	Integrations []string         `toml:"integrations,omitempty"`
}

// ToolRun builds an agentic run of the test tool, in its own approved work dir once it adds config.
type ToolRun struct {
	t          *testing.T
	proxy      *proxyTable
	flags      []string
	entrypoint bool
	dir        string
	container  string
}

// NewToolRun starts building a run in the shared work dir.
func NewToolRun(t *testing.T) *ToolRun {
	return &ToolRun{t: t}
}

// Allowing adds hosts to the proxy allowlist.
func (r *ToolRun) Allowing(hosts ...string) *ToolRun {
	proxy := r.proxyRules()
	proxy.AllowedHosts = append(proxy.AllowedHosts, hosts...)
	return r
}

// WithCredential injects the secret at secretPath for host as X-Api-Key, with EXAMPLE_API_KEY holding the placeholder.
func (r *ToolRun) WithCredential(host, secretPath string) *ToolRun {
	proxy := r.proxyRules()
	proxy.Credentials = append(proxy.Credentials, credentialRule{
		Hosts:  []string{host},
		Header: "X-Api-Key",
		Env:    []string{"EXAMPLE_API_KEY"},
		Secret: secretPath,
	})
	return r
}

// AllowingPaths limits host to paths.
func (r *ToolRun) AllowingPaths(host string, paths ...string) *ToolRun {
	proxy := r.proxyRules()
	proxy.AllowedPaths = append(proxy.AllowedPaths, pathRule{
		Host:  host,
		Paths: paths,
	})
	return r
}

// Integrating routes the named tools through the proxy.
func (r *ToolRun) Integrating(names ...string) *ToolRun {
	proxy := r.proxyRules()
	proxy.Integrations = append(proxy.Integrations, names...)
	return r
}

// WithProxy runs with --proxy.
func (r *ToolRun) WithProxy() *ToolRun {
	r.flags = append(r.flags, "--proxy")
	return r
}

// WithVolume adds a --volume mount.
func (r *ToolRun) WithVolume(spec string) *ToolRun {
	r.flags = append(r.flags, "--volume", spec)
	return r
}

// WithIntegrationFlag routes the named tools through the proxy with --proxy-integration.
func (r *ToolRun) WithIntegrationFlag(names ...string) *ToolRun {
	r.flags = append(r.flags, "--proxy-integration", strings.Join(names, ","))
	return r
}

// ViaEntrypoint runs the script through the image entrypoint, which -- skips and which makes the tool trust the proxy CA.
func (r *ToolRun) ViaEntrypoint() *ToolRun {
	r.entrypoint = true
	return r
}

// Run runs script with sh inside the tool container and returns the combined output.
func (r *ToolRun) Run(script string) string {
	r.t.Helper()
	args := r.args(script)

	out, err := AgenticCmdIn(r.workDir(), args...).CombinedOutput()
	r.t.Logf("agentic %s\n%s", strings.Join(args, " "), out)
	require.NoError(r.t, err)
	return string(out)
}

// During calls probe once the tool is up, then lets the tool exit and checks the run succeeded; for checks on the sidecars,
// which only live as long as the run.
func (r *ToolRun) During(probe func()) {
	r.t.Helper()
	cmd, out := r.Start()

	// Deferred, so the tool also exits when probe fails the test
	defer func() { assert.NoError(r.t, r.stop(cmd, out)) }()
	probe()
}

// Start starts the run and waits until the tool is up; the tool keeps running until stop.
func (r *ToolRun) Start() (*exec.Cmd, *bytes.Buffer) {
	r.t.Helper()
	ready := filepath.Join(r.workDir(), ".ready")
	_ = os.Remove(ready)
	_ = os.Remove(filepath.Join(r.dir, ".done"))

	cmd := AgenticCmdIn(r.dir, r.args(waitScript)...)
	out := &bytes.Buffer{}
	cmd.Stdout = out
	cmd.Stderr = out
	require.NoError(r.t, cmd.Start())

	started := assert.Eventually(r.t, func() bool {
		_, err := os.Stat(ready)
		return err == nil
	}, 2*time.Minute, 100*time.Millisecond, "tool never started")
	if !started {
		_ = r.stop(cmd, out)
		r.t.FailNow()
	}

	id, err := os.ReadFile(ready)
	require.NoError(r.t, err)
	name, err := exec.Command("docker", "inspect", "--format", "{{.Name}}", strings.TrimSpace(string(id))).Output()
	require.NoError(r.t, err)
	r.container = strings.TrimPrefix(strings.TrimSpace(string(name)), "/")
	return cmd, out
}

// Containers returns the names of a started run's tool container and its sidecars, running or not.
func (r *ToolRun) Containers() []string {
	r.t.Helper()
	return append([]string{r.container}, r.sidecars("--all")...)
}

// Proxy returns the name of a started run's running proxy sidecar.
func (r *ToolRun) Proxy() string {
	r.t.Helper()
	names := r.sidecars("--filter", "name=agentic-proxy-")
	require.Len(r.t, names, 1, "expected one running proxy")
	return names[0]
}

// stop lets a started tool exit, logs the run's output and returns its error.
func (r *ToolRun) stop(cmd *exec.Cmd, out *bytes.Buffer) error {
	r.t.Helper()
	_ = os.WriteFile(filepath.Join(r.dir, ".done"), nil, 0o644)
	err := cmd.Wait()
	r.t.Logf("agentic %s\n%s", strings.Join(cmd.Args[1:], " "), out)
	return err
}

// sidecars returns the names of the containers owned by the run's tool container that docker ps lists with the extra args.
func (r *ToolRun) sidecars(args ...string) []string {
	r.t.Helper()
	args = append([]string{"ps", "--filter", "label=" + labelOwner + "=" + r.container, "--format", "{{.Names}}"}, args...)
	out, err := exec.Command("docker", args...).Output()
	require.NoError(r.t, err)
	return strings.Fields(string(out))
}

// args returns the agentic run args running script with sh in the test tool, through the entrypoint if asked.
func (r *ToolRun) args(script string) []string {
	if r.entrypoint {
		script = entrypointScript(script)
	}
	args := append([]string{"run", "--trust-dir"}, r.flags...)
	return append(args, Tool, "--", "sh", "-c", script)
}

// workDir returns the run's work dir: the shared one without extra config, else a temp dir with an approved .agenticrc.toml.
func (r *ToolRun) workDir() string {
	r.t.Helper()
	if r.dir != "" {
		return r.dir
	}
	if r.proxy == nil {
		r.dir = workDir
		return r.dir
	}

	dir := TempDirIn(r.t, rootDir)
	rcPath := filepath.Join(dir, ".agenticrc.toml")
	require.NoError(r.t, os.WriteFile(rcPath, r.rc(), 0o644))
	require.NoError(r.t, approveSettings(rcPath))
	r.dir = dir
	return r.dir
}

// rc returns rcContents with the run's proxy table added.
func (r *ToolRun) rc() []byte {
	r.t.Helper()
	var rc map[string]any
	_, err := toml.Decode(rcContents, &rc)
	require.NoError(r.t, err)
	rc["run"].(map[string]any)["proxy"] = r.proxy

	var out bytes.Buffer
	require.NoError(r.t, toml.NewEncoder(&out).Encode(rc))
	return out.Bytes()
}

// proxyRules returns the run's [run.proxy] table, creating it on first use.
func (r *ToolRun) proxyRules() *proxyTable {
	if r.proxy == nil {
		r.proxy = &proxyTable{}
	}
	return r.proxy
}

// entrypointScript runs the image entrypoint with a claude shim on the PATH that runs script.
func entrypointScript(script string) string {
	return "mkdir -p /tmp/shim\n" +
		"cat > /tmp/shim/claude <<'EOF'\n#!/bin/sh\n" + script + "\nEOF\n" +
		"chmod +x /tmp/shim/claude\n" +
		"PATH=/tmp/shim:$PATH exec /usr/local/bin/entrypoint.sh"
}
