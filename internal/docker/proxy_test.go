package docker

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/certs"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartProxy(t *testing.T) {
	client := newTestClient()

	stubProxySettleTime(t, 0)
	creds := []sandbox.Credential{{Name: "EXAMPLE_API_KEY", Hosts: []string{"api.example.test"}, Headers: []string{"X-Api-Key"}, Secret: "test-secret"}}

	t.Run("creates, then boots the proxy and keeps its CA", func(t *testing.T) {
		// Arrange
		get := stubProxyDocker(t, client, "true")
		stubDockerRunWithStdinCapture(t, client, nil)
		rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Credentials: creds, Proxy: sandbox.ProxySpec{LogDir: t.TempDir()}}, network: "agentic-run-abc"}

		// Act
		handle, err := client.startProxy(rs)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, findCall(get(), "create"))
		assert.NotNil(t, findCall(get(), "network", "connect", NetworkName, handle.container))
		assert.NotEmpty(t, handle.caPEM)
	})

	t.Run("removes the proxy when it fails to boot", func(t *testing.T) {
		// Arrange
		get := stubProxyDocker(t, client, "true")
		stubDockerRunWithStdinCapture(t, client, fmt.Errorf("stub: cp failed"))
		rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Credentials: creds, Proxy: sandbox.ProxySpec{LogDir: t.TempDir()}}, network: "agentic-run-abc"}

		// Act
		_, err := client.startProxy(rs)

		// Assert
		require.ErrorContains(t, err, "copy proxy credentials")
		assert.NotNil(t, findCall(get(), "rm", "--force", "--volumes"))
	})

	t.Run("has nothing to remove when the create fails", func(t *testing.T) {
		// Arrange
		get := stubProxyDocker(t, client, "true", "create")
		rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Proxy: sandbox.ProxySpec{LogDir: t.TempDir()}}, network: "agentic-run-abc"}

		// Act
		_, err := client.startProxy(rs)

		// Assert
		require.ErrorContains(t, err, "create proxy")
		assert.Nil(t, findCall(get(), "rm"))
		assert.Nil(t, findCall(get(), "network", "rm"), "setupSidecars owns the run network")
	})
}

func TestClient_createProxy(t *testing.T) {
	client := newTestClient()

	creds := []sandbox.Credential{{Name: "EXAMPLE_API_KEY", Hosts: []string{"api.example.test"}, Headers: []string{"X-Api-Key"}, Secret: "test-secret"}}

	t.Run("creates a hardened sidecar on the run network without starting it", func(t *testing.T) {
		// Arrange
		get := stubProxyDocker(t, client, "true")
		rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Proxy: sandbox.ProxySpec{Allow: []string{"api.anthropic.com"}, LogDir: t.TempDir()}}, container: "agentic-claude-abc", network: "agentic-run-abc"}

		// Act
		handle, err := client.createProxy(rs)

		// Assert
		require.NoError(t, err)
		calls := get()
		createArgs := findCall(calls, "create")

		assert.Nil(t, findCall(calls, "network", "create"), "setupSidecars creates the run network")
		assert.Nil(t, findCall(calls, "start"))

		assert.Contains(t, createArgs, "--network=agentic-run-abc")
		assert.Contains(t, createArgs, "--read-only")
		assert.Contains(t, createArgs, "--network-alias=agentic-proxy")
		assert.Contains(t, createArgs, "--cap-drop=ALL")
		assert.Contains(t, createArgs, "--security-opt=no-new-privileges:true")
		assert.Contains(t, createArgs, "--user="+testUser.String(), "runs as the tool's user, who owns its credential files")
		assert.Contains(t, createArgs, "--env=AGENTIC_PROXY_ALLOW=api.anthropic.com")
		assert.Contains(t, createArgs, "--env=AGENTIC_PROXY_MONITOR=false")
		assert.Contains(t, createArgs, "--env=AGENTIC_PROXY_LOG="+proxyLogMountDir+"/"+proxy.LogFilePrefix+handle.id+".jsonl")
		assert.Contains(t, createArgs, "--volume="+handle.logPath+":"+proxyLogMountDir+"/"+proxy.LogFilePrefix+handle.id+".jsonl", "only the run's own log file is mounted")
		assert.FileExists(t, handle.logPath)
		assert.True(t, hasArgWithPrefix(createArgs, "--env=AGENTIC_PROXY_TZ_OFFSET="))
		assert.Contains(t, createArgs, "--label=agentic.owner=agentic-claude-abc")
		assert.True(t, hasArgWithPrefix(createArgs, "--label=agentic.started="))
		assert.Equal(t, tools.ProxyImage, createArgs[len(createArgs)-1])
	})

	t.Run("restricts clients to the run network's subnets", func(t *testing.T) {
		// Arrange
		get := stubProxyDocker(t, client, "true")
		rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Proxy: sandbox.ProxySpec{LogDir: t.TempDir()}}, network: "agentic-run-abc"}

		// Act
		_, err := client.createProxy(rs)

		// Assert
		require.NoError(t, err)
		calls := get()
		assert.NotNil(t, findCall(calls, "network", "inspect", "--format={{range .IPAM.Config}}{{.Subnet}} {{end}}", "agentic-run-abc"), "the subnets are the run network's")
		assert.Contains(t, findCall(calls, "create"), "--env="+proxy.EnvClientNets+"=172.30.0.0/16", "other containers on the egress network may not use the proxy")
	})

	t.Run("credentials add their files and an anonymous volume", func(t *testing.T) {
		// Arrange
		get := stubProxyDocker(t, client, "true")
		rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Credentials: creds, Proxy: sandbox.ProxySpec{LogDir: t.TempDir()}}, network: "agentic-run-abc"}

		// Act
		_, err := client.createProxy(rs)

		// Assert
		require.NoError(t, err)
		createArgs := findCall(get(), "create")
		assert.Contains(t, createArgs, "--env=AGENTIC_PROXY_CA_DIR="+proxyCredentialsDir)
		assert.Contains(t, createArgs, "--env=AGENTIC_PROXY_CREDENTIALS="+proxyCredentialsDir+"/"+proxyCredentialsFile)
		assert.Contains(t, createArgs, "--volume="+proxyRunMountDir, "an anonymous volume has no host path")
		for _, arg := range createArgs {
			assert.NotContains(t, arg, "test-secret")
		}
	})

	t.Run("without credentials nothing is mounted", func(t *testing.T) {
		// Arrange
		get := stubProxyDocker(t, client, "true")
		rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Proxy: sandbox.ProxySpec{LogDir: t.TempDir()}}, network: "agentic-run-abc"}

		// Act
		_, err := client.createProxy(rs)

		// Assert
		require.NoError(t, err)
		createArgs := findCall(get(), "create")
		assert.False(t, hasArgWithPrefix(createArgs, "--env=AGENTIC_PROXY_CA_DIR="))
		assert.NotContains(t, createArgs, "--volume="+proxyRunMountDir)
	})

	t.Run("creates nothing when the run network's subnets can't be read", func(t *testing.T) {
		// Arrange
		var calls []dockerCall
		stubDockerRun(t, client, func(args ...string) (string, error) {
			calls = append(calls, dockerCall{args: args})
			if args[len(args)-1] == NetworkName {
				return LabelProjectVal + "\n", nil
			}
			return "", nil
		})
		rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Proxy: sandbox.ProxySpec{LogDir: t.TempDir()}}, network: "agentic-run-abc"}

		// Act
		_, err := client.createProxy(rs)

		// Assert
		require.ErrorContains(t, err, "has no subnet")
		assert.Nil(t, findCall(calls, "create"))
	})
}

func Test_proxyHandle_createLogFile(t *testing.T) {
	t.Run("creates an empty file", func(t *testing.T) {
		// Arrange
		h := proxyHandle{logPath: filepath.Join(t.TempDir(), "proxy_abc.jsonl")}

		// Act
		err := h.createLogFile()

		// Assert
		require.NoError(t, err)
		got, err := os.ReadFile(h.logPath)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("existing file is refused", func(t *testing.T) {
		// Arrange
		h := proxyHandle{logPath: filepath.Join(t.TempDir(), "proxy_abc.jsonl")}
		require.NoError(t, os.WriteFile(h.logPath, nil, 0o600))

		// Act
		err := h.createLogFile()

		// Assert
		assert.ErrorContains(t, err, "create proxy log")
	})
}

func TestNewProxyHandle(t *testing.T) {
	client := newTestClient()

	t.Run("refuses a spec without a host user", func(t *testing.T) {
		// Act
		_, err := client.newProxyHandle(toolRun{RunSpec: sandbox.RunSpec{Proxy: sandbox.ProxySpec{Mode: sandbox.ProxyEnforce}}})

		// Assert
		assert.ErrorIs(t, err, errNoHostUser)
	})

	t.Run("log path is under the log dir", func(t *testing.T) {
		// Arrange
		rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Proxy: sandbox.ProxySpec{LogDir: "/tmp/agentic/logs"}}}

		// Act
		handle, err := client.newProxyHandle(rs)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(rs.Proxy.LogDir, proxy.LogFilePrefix+handle.id+".jsonl"), handle.logPath)
	})

	t.Run("enforce mode does not monitor", func(t *testing.T) {
		// Arrange
		rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Proxy: sandbox.ProxySpec{Mode: sandbox.ProxyEnforce}}}

		// Act
		handle, err := client.newProxyHandle(rs)

		// Assert
		require.NoError(t, err)
		assert.False(t, handle.monitor)
	})

	t.Run("monitor mode monitors", func(t *testing.T) {
		// Arrange
		rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Proxy: sandbox.ProxySpec{Mode: sandbox.ProxyMonitor}}}

		// Act
		handle, err := client.newProxyHandle(rs)

		// Assert
		require.NoError(t, err)
		assert.True(t, handle.monitor)
	})
}

func TestProxyHandleStop(t *testing.T) {
	client := newTestClient()

	t.Run("removes only the container", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client)
		handle := proxyHandle{client: client, container: "agentic-proxy-abc"}

		// Act
		handle.Stop()

		// Assert
		calls := get()
		require.Len(t, calls, 1, "the run network outlives the proxy")
		assert.Equal(t, []string{"rm", "--force", "--volumes", "agentic-proxy-abc"}, calls[0].args)
	})
}

func Test_proxyEnvArgs(t *testing.T) {
	t.Run("points at the proxy and excludes loopback", func(t *testing.T) {
		// Act
		args := proxyEnvArgs(false)

		// Assert
		assert.Contains(t, args, "--env=HTTPS_PROXY=http://agentic-proxy:3128")
		assert.Contains(t, args, "--env=HTTP_PROXY=http://agentic-proxy:3128")
		assert.Contains(t, args, "--env=NO_PROXY=localhost,127.0.0.1")
	})

	t.Run("dind excludes the docker sidecar", func(t *testing.T) {
		// Act
		args := proxyEnvArgs(true)

		// Assert
		assert.Contains(t, args, "--env=NO_PROXY=localhost,127.0.0.1,agentic-docker")
	})
}

func TestProxyHandleHostsMatching(t *testing.T) {
	t.Run("collects unique denied hosts and total", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		logPath := filepath.Join(dir, "run.jsonl")
		lines := strings.Join([]string{
			`{"time":"2026-06-17T12:00:00Z","host":"api.anthropic.com","port":"443","decision":"allow"}`,
			`{"time":"2026-06-17T12:00:01Z","host":"evil.com","port":"443","decision":"deny"}`,
			`{"time":"2026-06-17T12:00:02Z","host":"evil.com","port":"443","decision":"deny"}`,
			`{"time":"2026-06-17T12:00:03Z","host":"tracker.net","port":"443","decision":"deny"}`,
		}, "\n")
		require.NoError(t, os.WriteFile(logPath, []byte(lines+"\n"), 0o644))
		handle := proxyHandle{logPath: logPath}

		// Act
		hosts, total := handle.hostsMatching(deniedByAllowlist)

		// Assert
		assert.Equal(t, []string{"evil.com", "tracker.net"}, hosts)
		assert.Equal(t, 3, total)
	})

	t.Run("a denied host whose upstream failed in monitor mode still counts", func(t *testing.T) {
		// Arrange
		logPath := filepath.Join(t.TempDir(), "run.jsonl")
		lines := strings.Join([]string{
			`{"time":"2026-06-17T12:00:00Z","host":"down.example.test","port":"443","decision":"deny","reason":"upstream-failed"}`,
			`{"time":"2026-06-17T12:00:01Z","host":"local.example.test","port":"443","decision":"deny","reason":"blocked-address"}`,
		}, "\n")
		require.NoError(t, os.WriteFile(logPath, []byte(lines+"\n"), 0o644))
		handle := proxyHandle{logPath: logPath}

		// Act
		hosts, total := handle.hostsMatching(deniedByAllowlist)

		// Assert
		assert.Equal(t, []string{"down.example.test"}, hosts)
		assert.Equal(t, 1, total)
	})

	t.Run("hosts are made safe to print", func(t *testing.T) {
		// Arrange
		logPath := filepath.Join(t.TempDir(), "run.jsonl")
		line := `{"time":"2026-06-17T12:00:00Z","host":"evil\u202e.test","port":"443","decision":"deny"}`
		require.NoError(t, os.WriteFile(logPath, []byte(line+"\n"), 0o644))
		handle := proxyHandle{logPath: logPath}

		// Act
		hosts, _ := handle.hostsMatching(deniedByAllowlist)

		// Assert
		assert.Equal(t, []string{`"evil\u202e.test"`}, hosts)
	})

	t.Run("missing log yields nothing", func(t *testing.T) {
		// Arrange
		handle := proxyHandle{logPath: filepath.Join(t.TempDir(), "absent.jsonl")}

		// Act
		hosts, total := handle.hostsMatching(deniedByAllowlist)

		// Assert
		assert.Empty(t, hosts)
		assert.Zero(t, total)
	})
}

func TestProxyHandlePrintSummary(t *testing.T) {
	writeLog := func(t *testing.T, lines ...string) string {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "run.jsonl")
		require.NoError(t, os.WriteFile(logPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644))
		return logPath
	}

	t.Run("enforced mode reports blocked hosts", func(t *testing.T) {
		// Arrange
		logPath := writeLog(t,
			`{"host":"api.anthropic.com","decision":"allow"}`,
			`{"host":"evil.com","decision":"deny"}`,
		)
		handle := proxyHandle{logPath: logPath}
		var buf strings.Builder

		// Act
		handle.PrintSummary(logging.New(&buf))

		// Assert
		assert.Contains(t, buf.String(), "agentic: proxy blocked 1 request(s) to: evil.com")
		assert.NotContains(t, buf.String(), "monitor mode")
	})

	t.Run("monitor mode reports would-be-blocked hosts", func(t *testing.T) {
		// Arrange
		logPath := writeLog(t,
			`{"host":"api.anthropic.com","decision":"allow"}`,
			`{"host":"evil.com","decision":"deny"}`,
		)
		handle := proxyHandle{logPath: logPath, monitor: true}
		var buf strings.Builder

		// Act
		handle.PrintSummary(logging.New(&buf))

		// Assert
		assert.Contains(t, buf.String(), "agentic: proxy (monitor mode) observed 2 request(s); 1 would be blocked under the current allowlist: evil.com")
		assert.Contains(t, buf.String(), "--proxy-monitor")
	})

	t.Run("warns about credential hosts reached without the placeholder", func(t *testing.T) {
		// Arrange
		logPath := writeLog(t,
			`{"host":"api.example.test","decision":"allow","intercepted":true}`,
			`{"host":"api.example.test","decision":"allow","intercepted":true,"reason":"no-placeholder"}`,
		)
		handle := proxyHandle{logPath: logPath}
		var buf strings.Builder

		// Act
		handle.PrintSummary(logging.New(&buf))

		// Assert
		assert.Contains(t, buf.String(), "agentic: warning: proxy saw 1 connection(s) to credential hosts without the placeholder, so they likely ran unauthenticated: api.example.test")
		assert.NotContains(t, buf.String(), "blocked")
	})

	t.Run("monitor mode doesn't count a missed placeholder as another request or block", func(t *testing.T) {
		// Arrange
		logPath := writeLog(t,
			`{"host":"evil.com","decision":"deny"}`,
			`{"host":"api.example.test","decision":"deny","intercepted":true,"reason":"no-placeholder"}`,
		)
		handle := proxyHandle{logPath: logPath, monitor: true}
		var buf strings.Builder

		// Act
		handle.PrintSummary(logging.New(&buf))

		// Assert
		assert.Contains(t, buf.String(), "observed 1 request(s); 1 would be blocked under the current allowlist: evil.com")
	})

	t.Run("monitor mode counts a tunnel with failed requests once", func(t *testing.T) {
		// Arrange
		logPath := writeLog(t,
			`{"host":"evil.com","decision":"deny"}`,
			`{"host":"api.example.test","decision":"allow","intercepted":true}`,
			`{"host":"api.example.test","decision":"allow","intercepted":true,"reason":"upstream-failed"}`,
		)
		handle := proxyHandle{logPath: logPath, monitor: true}
		var buf strings.Builder

		// Act
		handle.PrintSummary(logging.New(&buf))

		// Assert
		assert.Contains(t, buf.String(), "observed 2 request(s); 1 would be blocked under the current allowlist: evil.com")
	})

	t.Run("guard refusals are reported apart from allowlist blocks", func(t *testing.T) {
		// Arrange
		logPath := writeLog(t,
			`{"host":"evil.com","decision":"deny"}`,
			`{"host":"localhost","decision":"deny","reason":"blocked-address"}`,
		)
		handle := proxyHandle{logPath: logPath}
		var buf strings.Builder

		// Act
		handle.PrintSummary(logging.New(&buf))

		// Assert
		assert.Contains(t, buf.String(), "agentic: proxy blocked 1 request(s) to: evil.com")
		assert.Contains(t, buf.String(), "agentic: proxy refused 1 request(s) to local or private addresses: localhost")
	})

	t.Run("monitor mode reports guard refusals as refused, not would-be-blocked", func(t *testing.T) {
		// Arrange
		logPath := writeLog(t, `{"host":"localhost","decision":"deny","enforced":true,"reason":"blocked-address"}`)
		handle := proxyHandle{logPath: logPath, monitor: true}
		var buf strings.Builder

		// Act
		handle.PrintSummary(logging.New(&buf))

		// Assert
		assert.Contains(t, buf.String(), "agentic: proxy refused 1 request(s) to local or private addresses: localhost")
		assert.NotContains(t, buf.String(), "would be blocked")
	})

	t.Run("nothing denied prints nothing", func(t *testing.T) {
		// Arrange
		logPath := writeLog(t, `{"host":"api.anthropic.com","decision":"allow"}`)
		handle := proxyHandle{logPath: logPath, monitor: true}
		var buf strings.Builder

		// Act
		handle.PrintSummary(logging.New(&buf))

		// Assert
		assert.Empty(t, buf.String())
	})
}

func Test_setupProxy(t *testing.T) {
	client := newTestClient()

	t.Run("dry run makes no docker calls", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t, client)
		rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Image: "agentic-claude", DryRun: true, Proxy: sandbox.ProxySpec{Mode: sandbox.ProxyEnforce, LogDir: t.TempDir()}}}

		// Act
		env, cleanup, err := client.setupProxy(rs)

		// Assert
		require.NoError(t, err)
		assert.NotEmpty(t, env)
		assert.Empty(t, calls(), "dry run must not invoke docker")
		require.NotPanics(t, cleanup)
	})

	t.Run("cleanup stops the sidecar", func(t *testing.T) {
		// Arrange
		calls := stubProxyDocker(t, client, "true")
		rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Image: "agentic-claude", Proxy: sandbox.ProxySpec{Mode: sandbox.ProxyEnforce, LogDir: t.TempDir()}}}

		// Act
		_, cleanup, err := client.setupProxy(rs)
		cleanup()

		// Assert
		require.NoError(t, err)
		last := calls()[len(calls())-1]
		assert.Equal(t, []string{"rm", "--force", "--volumes"}, last.args[:3])
	})

	t.Run("propagates startProxy error", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, client, "network inspect", "network create")
		rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Image: "agentic-claude", Proxy: sandbox.ProxySpec{Mode: sandbox.ProxyEnforce, LogDir: t.TempDir()}}}

		// Act
		env, cleanup, err := client.setupProxy(rs)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, env)
		assert.Nil(t, cleanup)
	})
}

func Test_proxyHandle_boot(t *testing.T) {
	client := newTestClient()

	stubProxySettleTime(t, 0)
	creds := []sandbox.Credential{{Name: "EXAMPLE_API_KEY", Hosts: []string{"api.example.test"}, Headers: []string{"X-Api-Key"}, Secret: "test-secret"}}
	rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Credentials: creds}}

	t.Run("copies a scoped CA and the credentials in before start, then connects to the egress network", func(t *testing.T) {
		// Arrange
		var callsAtCopy int
		get := stubProxyDocker(t, client, "true")
		stdin := stubDockerRunWithStdinCapture(t, client, nil)
		f := fakeOf(client)
		orig := f.runWithStdin
		f.runWithStdin = func(r io.Reader, args ...string) (string, error) {
			callsAtCopy = len(get())
			return orig(r, args...)
		}
		handle, err := client.newProxyHandle(rs)
		require.NoError(t, err)

		// Act
		caPEM, err := handle.boot()

		// Assert
		require.NoError(t, err)
		calls := get()
		require.Len(t, stdin(), 1)
		assert.Equal(t, []string{"cp", "--archive", "-", handle.container + ":" + proxyRunMountDir}, stdin()[0].args)
		assert.Nil(t, findCall(calls[:callsAtCopy], "start", handle.container), "credentials must be in place before the proxy starts")
		assert.NotNil(t, findCall(calls[callsAtCopy:], "start", handle.container))
		assert.Equal(t, []string{"network", "connect", NetworkName, handle.container}, findCall(calls, "network", "connect"))

		files := readTar(t, stdin()[0].input)
		for _, name := range []string{proxy.CACertFile, proxy.CAKeyFile, proxyCredentialsFile} {
			entry := proxyCredentialsSubdir + "/" + name
			require.Contains(t, files, entry)
			assert.Equal(t, int64(0o600), files[entry].header.Mode, name)
			assert.Equal(t, testUser.UID, files[entry].header.Uid, name)
			assert.Equal(t, testUser.GID, files[entry].header.Gid, name)
		}

		var loaded []proxy.Credential
		require.NoError(t, json.Unmarshal(files[proxyCredentialsSubdir+"/"+proxyCredentialsFile].content, &loaded))
		assert.Equal(t, handle.credentials, loaded)
		require.Len(t, loaded, 1)
		assert.Equal(t, "test-secret", loaded[0].Secret)

		ca, err := certs.LoadCA(files[proxyCredentialsSubdir+"/"+proxy.CACertFile].content, files[proxyCredentialsSubdir+"/"+proxy.CAKeyFile].content)
		require.NoError(t, err)
		assert.True(t, ca.Scoped())
		assert.Equal(t, files[proxyCredentialsSubdir+"/"+proxy.CACertFile].content, caPEM, "the tool must trust the CA the proxy got")
	})

	t.Run("a failed copy starts nothing", func(t *testing.T) {
		// Arrange
		get := stubProxyDocker(t, client, "true")
		stubDockerRunWithStdinCapture(t, client, fmt.Errorf("stub: cp failed"))
		handle, err := client.newProxyHandle(rs)
		require.NoError(t, err)

		// Act
		_, err = handle.boot()

		// Assert
		require.ErrorContains(t, err, "copy proxy credentials")
		assert.Nil(t, findCall(get(), "start"))
	})

	t.Run("a proxy that exits on start isn't connected", func(t *testing.T) {
		// Arrange
		get := stubProxyDocker(t, client, "false")
		stubDockerRunWithStdinCapture(t, client, nil)
		handle, err := client.newProxyHandle(rs)
		require.NoError(t, err)

		// Act
		_, err = handle.boot()

		// Assert
		require.ErrorContains(t, err, "proxy exited on start")
		assert.Nil(t, findCall(get(), "network", "connect"))
	})

	t.Run("without credentials nothing is copied", func(t *testing.T) {
		// Arrange
		get := stubProxyDocker(t, client, "true")
		stdin := stubDockerRunWithStdinCapture(t, client, nil)
		handle, err := client.newProxyHandle(toolRun{RunSpec: sandbox.RunSpec{User: &testUser}})
		require.NoError(t, err)

		// Act
		caPEM, err := handle.boot()

		// Assert
		require.NoError(t, err)
		assert.Empty(t, stdin())
		assert.Empty(t, caPEM)
		assert.NotNil(t, findCall(get(), "network", "connect", NetworkName, handle.container))
	})
}

func Test_proxyHandle_checkStarted(t *testing.T) {
	client := newTestClient()

	stubProxySettleTime(t, 0)
	creds := []proxy.Credential{{Hosts: []string{"api.example.test"}, Headers: []string{"X-Api-Key"}, Placeholder: "agentic-0123456789abcdef", Secret: "test-secret"}}

	t.Run("running proxy passes", func(t *testing.T) {
		// Arrange
		stubDockerRunBySubcmd(t, client, map[string]string{"inspect": "true\n"})
		handle := proxyHandle{client: client, container: "agentic-proxy-abc", credentials: creds}

		// Act
		err := handle.checkStarted()

		// Assert
		assert.NoError(t, err)
	})

	t.Run("exited proxy fails with its logs", func(t *testing.T) {
		// Arrange
		stubDockerRunBySubcmd(t, client, map[string]string{"inspect": "false\n", "logs": "load proxy CA: bad key\n"})
		handle := proxyHandle{client: client, container: "agentic-proxy-abc", credentials: creds}

		// Act
		err := handle.checkStarted()

		// Assert
		assert.ErrorContains(t, err, "proxy exited on start")
		assert.ErrorContains(t, err, "load proxy CA: bad key")
	})

	t.Run("without credentials nothing is checked", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client)
		handle := proxyHandle{client: client, container: "agentic-proxy-abc"}

		// Act
		err := handle.checkStarted()

		// Assert
		require.NoError(t, err)
		assert.Empty(t, get())
	})
}

func Test_dryRunProxy(t *testing.T) {
	client := newTestClient()

	t.Run("credentials print no secret and touch nothing", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t, client)
		stdin := stubDockerRunWithStdinCapture(t, client, nil)
		creds := []sandbox.Credential{{Name: "EXAMPLE_API_KEY", Hosts: []string{"api.example.test"}, Headers: []string{"X-Api-Key"}, Secret: "test-secret"}}
		rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, DryRun: true, Credentials: creds, Proxy: sandbox.ProxySpec{Mode: sandbox.ProxyEnforce, LogDir: t.TempDir()}}}

		// Act
		var toolArgs []string
		out := captureStdout(t, func() {
			var err error
			toolArgs, _, err = client.dryRunProxy(rs)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, toolArgs, "--env="+tools.ProxyCAEnvName+"="+dryRunCAPlaceholder)
		assert.Regexp(t, `--env=EXAMPLE_API_KEY=agentic-[0-9a-f]{24}`, strings.Join(toolArgs, " "))
		assert.Contains(t, out, "--volume="+proxyRunMountDir)
		assert.Contains(t, out, proxy.EnvClientNets+"="+dryRunClientNetsPlaceholder)
		assert.NotContains(t, out, "test-secret")
		assert.Empty(t, calls())
		assert.Empty(t, stdin())
	})
}

func Test_repeatsAttempt(t *testing.T) {
	cases := map[string]struct {
		entry proxy.Entry
		want  bool
	}{
		"intercepted tunnel":             {proxy.Entry{Intercepted: true}, false},
		"failed request inside a tunnel": {proxy.Entry{Intercepted: true, Reason: proxy.ReasonUpstreamFailed}, true},
		"missed placeholder":             {proxy.Entry{Intercepted: true, Reason: proxy.ReasonNoPlaceholder}, true},
		"failed plain tunnel":            {proxy.Entry{Reason: proxy.ReasonUpstreamFailed}, false},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			got := repeatsAttempt(c.entry)

			// Assert
			assert.Equal(t, c.want, got)
		})
	}
}

func TestSweepProxyResources(t *testing.T) {
	// Arrange
	client := newTestClient()
	get := stubUnusedSweep(t, client, "", ownedRow("agentic-proxy-abc", "agentic-claude-gone", time.Hour))

	// Act
	err := client.SweepProxyResources()

	// Assert
	require.NoError(t, err)
	calls := get()
	assert.Contains(t, calls[1].args, "--filter=name=agentic-proxy")
	assert.Equal(t, []string{"rm", "--force", "--volumes", "agentic-proxy-abc"}, findCall(calls, "rm"))
}
