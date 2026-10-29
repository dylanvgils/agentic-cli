//go:build integration

package integration

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

// waitScript signals the tool is up, then keeps it running until .done exists.
const waitScript = "touch .ready; while [ ! -f .done ]; do sleep 0.1; done"

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

// toolRun builds an agentic run of the test tool, in its own approved work dir once it adds config.
type toolRun struct {
	t          *testing.T
	proxy      *proxyTable
	flags      []string
	entrypoint bool
	dir        string
}

// newToolRun starts building a run in the shared work dir.
func newToolRun(t *testing.T) *toolRun {
	return &toolRun{t: t}
}

// allowing adds hosts to the proxy allowlist.
func (r *toolRun) allowing(hosts ...string) *toolRun {
	proxy := r.proxyRules()
	proxy.AllowedHosts = append(proxy.AllowedHosts, hosts...)
	return r
}

// withCredential injects the secret at secretPath for host as X-Api-Key, with EXAMPLE_API_KEY holding the placeholder.
func (r *toolRun) withCredential(host, secretPath string) *toolRun {
	proxy := r.proxyRules()
	proxy.Credentials = append(proxy.Credentials, credentialRule{
		Hosts:  []string{host},
		Header: "X-Api-Key",
		Env:    []string{"EXAMPLE_API_KEY"},
		Secret: secretPath,
	})
	return r
}

// allowingPaths limits host to paths.
func (r *toolRun) allowingPaths(host string, paths ...string) *toolRun {
	proxy := r.proxyRules()
	proxy.AllowedPaths = append(proxy.AllowedPaths, pathRule{
		Host:  host,
		Paths: paths,
	})
	return r
}

// integrating routes the named tools through the proxy.
func (r *toolRun) integrating(names ...string) *toolRun {
	proxy := r.proxyRules()
	proxy.Integrations = append(proxy.Integrations, names...)
	return r
}

// withProxy runs with --proxy.
func (r *toolRun) withProxy() *toolRun {
	r.flags = append(r.flags, "--proxy")
	return r
}

// withVolume adds a --volume mount.
func (r *toolRun) withVolume(spec string) *toolRun {
	r.flags = append(r.flags, "--volume", spec)
	return r
}

// withIntegrationFlag routes the named tools through the proxy with --proxy-integration.
func (r *toolRun) withIntegrationFlag(names ...string) *toolRun {
	r.flags = append(r.flags, "--proxy-integration", strings.Join(names, ","))
	return r
}

// viaEntrypoint runs the script through the image entrypoint, which -- skips and which makes the tool trust the proxy CA.
func (r *toolRun) viaEntrypoint() *toolRun {
	r.entrypoint = true
	return r
}

// run runs script with sh inside the tool container and returns the combined output.
func (r *toolRun) run(script string) string {
	r.t.Helper()
	args := r.args(script)

	out, err := agenticCmdIn(r.workDir(), args...).CombinedOutput()
	r.t.Logf("agentic %s\n%s", strings.Join(args, " "), out)
	require.NoError(r.t, err)
	return string(out)
}

// during calls probe once the tool is up, then lets the tool exit and checks the run succeeded; for checks on the sidecars,
// which only live as long as the run.
func (r *toolRun) during(probe func()) {
	r.t.Helper()
	cmd, out := r.start()

	// Deferred, so the tool also exits when probe fails the test
	defer func() { assert.NoError(r.t, r.stop(cmd, out)) }()
	probe()
}

// start starts the run and waits until the tool is up; the tool keeps running until stop.
func (r *toolRun) start() (*exec.Cmd, *bytes.Buffer) {
	r.t.Helper()
	ready := filepath.Join(r.workDir(), ".ready")
	_ = os.Remove(ready)
	_ = os.Remove(filepath.Join(r.dir, ".done"))

	cmd := agenticCmdIn(r.dir, r.args(waitScript)...)
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
	return cmd, out
}

// stop lets a started tool exit, logs the run's output and returns its error.
func (r *toolRun) stop(cmd *exec.Cmd, out *bytes.Buffer) error {
	r.t.Helper()
	_ = os.WriteFile(filepath.Join(r.dir, ".done"), nil, 0o644)
	err := cmd.Wait()
	r.t.Logf("agentic %s\n%s", strings.Join(cmd.Args[1:], " "), out)
	return err
}

// args returns the agentic run args running script with sh in the test tool, through the entrypoint if asked.
func (r *toolRun) args(script string) []string {
	if r.entrypoint {
		script = entrypointScript(script)
	}
	args := append([]string{"run", "--trust-dir"}, r.flags...)
	return append(args, testTool, "--", "sh", "-c", script)
}

// workDir returns the run's work dir: the shared one without extra config, else a temp dir with an approved .agenticrc.toml.
func (r *toolRun) workDir() string {
	r.t.Helper()
	if r.dir != "" {
		return r.dir
	}
	if r.proxy == nil {
		r.dir = workDir
		return r.dir
	}

	dir := tempDirIn(r.t, rootDir)
	rcPath := filepath.Join(dir, ".agenticrc.toml")
	require.NoError(r.t, os.WriteFile(rcPath, r.rc(), 0o644))
	require.NoError(r.t, approveSettings(rcPath))
	r.dir = dir
	return r.dir
}

// rc returns rcContents with the run's proxy table added.
func (r *toolRun) rc() []byte {
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
func (r *toolRun) proxyRules() *proxyTable {
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
