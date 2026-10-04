package docker

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/certs"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartProxy(t *testing.T) {
	t.Run("creates internal network, hardened sidecar, and egress link", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, "network inspect")
		rs := RunSpec{container: "agentic-claude-abc", Proxy: ProxySpec{Image: "default-proxy", Allow: []string{"api.anthropic.com"}, LogDir: "/tmp/agentic/proxy"}}

		// Act
		handle, err := startProxy(rs)

		// Assert
		require.NoError(t, err)
		calls := get()

		var createArgs, runArgs, connectArgs []string
		for _, c := range calls {
			switch {
			case c.args[0] == "network" && c.args[1] == "create":
				createArgs = c.args
			case c.args[0] == "create":
				runArgs = c.args
			case c.args[0] == "network" && c.args[1] == "connect":
				connectArgs = c.args
			}
		}

		assert.Contains(t, createArgs, "--internal")
		assert.Contains(t, createArgs, handle.network)
		assert.Contains(t, createArgs, "--label=agentic.owner=agentic-claude-abc", "the network is swept with its owner")

		assert.Contains(t, runArgs, "--read-only")
		assert.Contains(t, runArgs, "--network-alias=agentic-proxy")
		assert.Contains(t, runArgs, "--cap-drop=ALL")
		assert.Contains(t, runArgs, "--security-opt=no-new-privileges:true")
		assert.Contains(t, runArgs, "--env=AGENTIC_PROXY_ALLOW=api.anthropic.com")
		assert.Contains(t, runArgs, "--env=AGENTIC_PROXY_MONITOR=false")
		assert.Contains(t, runArgs, "--env=AGENTIC_PROXY_LOG="+proxyLogMountDir+"/"+proxy.LogFilePrefix+handle.id+".jsonl")
		assert.True(t, hasArgWithPrefix(runArgs, "--env=AGENTIC_PROXY_TZ_OFFSET="))
		assert.Contains(t, runArgs, "--label=agentic.owner=agentic-claude-abc")
		assert.True(t, hasArgWithPrefix(runArgs, "--label=agentic.started="))
		assert.Equal(t, "default-proxy", runArgs[len(runArgs)-1])

		assert.Equal(t, []string{"network", "connect", NetworkName, handle.container}, connectArgs)
	})

	t.Run("removes network when sidecar fails to start", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, "network inspect", "create")
		rs := RunSpec{Proxy: ProxySpec{Image: "default-proxy", LogDir: "/tmp/agentic/proxy"}}

		// Act
		_, err := startProxy(rs)

		// Assert
		require.Error(t, err)
		var sawNetworkRm bool
		for _, c := range get() {
			if c.args[0] == "network" && c.args[1] == "rm" {
				sawNetworkRm = true
			}
		}
		assert.True(t, sawNetworkRm, "expected network rm cleanup after failed run")
	})
}

func TestNewProxyHandle(t *testing.T) {
	t.Run("log path is under the log dir", func(t *testing.T) {
		// Arrange
		rs := RunSpec{Proxy: ProxySpec{LogDir: "/tmp/agentic/logs"}}

		// Act
		handle, err := newProxyHandle(rs)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(rs.Proxy.LogDir, proxy.LogFilePrefix+handle.id+".jsonl"), handle.logPath)
	})

	t.Run("enforce mode does not monitor", func(t *testing.T) {
		// Arrange
		rs := RunSpec{Proxy: ProxySpec{Mode: ProxyEnforce}}

		// Act
		handle, err := newProxyHandle(rs)

		// Assert
		require.NoError(t, err)
		assert.False(t, handle.monitor)
	})

	t.Run("monitor mode monitors", func(t *testing.T) {
		// Arrange
		rs := RunSpec{Proxy: ProxySpec{Mode: ProxyMonitor}}

		// Act
		handle, err := newProxyHandle(rs)

		// Assert
		require.NoError(t, err)
		assert.True(t, handle.monitor)
	})
}

func TestProxyMode_Enabled(t *testing.T) {
	t.Run("zero value is off", func(t *testing.T) {
		// Arrange
		var mode ProxyMode

		// Act
		result := mode.Enabled()

		// Assert
		assert.Equal(t, ProxyOff, mode)
		assert.False(t, result)
	})

	t.Run("enforce is enabled", func(t *testing.T) {
		// Act
		result := ProxyEnforce.Enabled()

		// Assert
		assert.True(t, result)
	})

	t.Run("monitor is enabled", func(t *testing.T) {
		// Act
		result := ProxyMonitor.Enabled()

		// Assert
		assert.True(t, result)
	})
}

func TestProxyHandleStop(t *testing.T) {
	t.Run("removes container and network", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t)
		handle := proxyHandle{container: "agentic-proxy-abc", network: "agentic-proxy-abc"}

		// Act
		handle.Stop()

		// Assert
		calls := get()
		require.Len(t, calls, 2)
		assert.Equal(t, []string{"rm", "--force", "--volumes", "agentic-proxy-abc"}, calls[0].args)
		assert.Equal(t, []string{"network", "rm", "agentic-proxy-abc"}, calls[1].args)
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

func TestProxyHandleHostsByDecision(t *testing.T) {
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
		hosts, total := handle.hostsByDecision(proxy.DecisionDeny)

		// Assert
		assert.Equal(t, []string{"evil.com", "tracker.net"}, hosts)
		assert.Equal(t, 3, total)
	})

	t.Run("missing log yields nothing", func(t *testing.T) {
		// Arrange
		handle := proxyHandle{logPath: filepath.Join(t.TempDir(), "absent.jsonl")}

		// Act
		hosts, total := handle.hostsByDecision(proxy.DecisionDeny)

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
	t.Run("dry run sets network without docker calls", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t)
		rs := RunSpec{Image: "agentic-claude", DryRun: true, Proxy: ProxySpec{Mode: ProxyEnforce, LogDir: t.TempDir()}}

		// Act
		env, cleanup, err := setupProxy(&rs)

		// Assert
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(rs.network, "agentic-proxy-"))
		assert.NotEmpty(t, env)
		assert.Empty(t, calls(), "dry run must not invoke docker")
		require.NotPanics(t, cleanup)
	})

	t.Run("cleanup stops the sidecar", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t, "network inspect")
		rs := RunSpec{Image: "agentic-claude", Proxy: ProxySpec{Mode: ProxyEnforce, Image: "default-proxy", LogDir: t.TempDir()}}

		// Act
		_, cleanup, err := setupProxy(&rs)
		cleanup()

		// Assert
		require.NoError(t, err)
		last := calls()[len(calls())-1]
		assert.Equal(t, []string{"network", "rm", rs.network}, last.args)
	})

	t.Run("propagates startProxy error", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, "network inspect", "network create")
		rs := RunSpec{Image: "agentic-claude", Proxy: ProxySpec{Mode: ProxyEnforce, Image: "default-proxy", LogDir: t.TempDir()}}

		// Act
		env, cleanup, err := setupProxy(&rs)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, env)
		assert.Nil(t, cleanup)
	})
}

func TestStartProxy_credentials(t *testing.T) {
	stubProxySettleTime(t, 0)
	creds := []proxy.Credential{{Hosts: []string{"api.example.test"}, Rules: []proxy.InjectRule{{Header: "X-Api-Key", Value: "test-secret"}}}}

	t.Run("copies a scoped CA and the credentials into the volume before start", func(t *testing.T) {
		// Arrange
		var callsAtCopy int
		get := stubProxyDocker(t, "true")
		stdin := stubDockerRunStdinCapture(t, nil)
		orig := dockerRunStdin
		dockerRunStdin = func(r io.Reader, args ...string) (string, error) {
			callsAtCopy = len(get())
			return orig(r, args...)
		}
		rs := RunSpec{Proxy: ProxySpec{Image: "default-proxy", LogDir: t.TempDir(), Credentials: creds}}

		// Act
		handle, err := startProxy(rs)

		// Assert
		require.NoError(t, err)
		calls := get()
		createArgs := findCall(calls, "create")
		assert.Contains(t, createArgs, "--env=AGENTIC_PROXY_CA_DIR="+proxyCredentialsDir)
		assert.Contains(t, createArgs, "--env=AGENTIC_PROXY_CREDENTIALS="+proxyCredentialsDir+"/"+proxyCredentialsFile)
		assert.Contains(t, createArgs, "--env=AGENTIC_PROXY_CLIENT_NETS=172.30.0.0/16", "only the run's own network may use the credentials")
		assert.Contains(t, createArgs, "--volume="+proxyRunMountDir, "an anonymous volume has no host path")
		for _, arg := range createArgs {
			assert.NotContains(t, arg, "test-secret")
		}

		require.Len(t, stdin(), 1)
		assert.Equal(t, []string{"cp", "--archive", "-", handle.container + ":" + proxyRunMountDir}, stdin()[0].args)
		assert.Nil(t, findCall(calls[:callsAtCopy], "start", handle.container), "credentials must be in place before the proxy starts")
		assert.NotNil(t, findCall(calls[callsAtCopy:], "start", handle.container))

		files := readTar(t, stdin()[0].input)
		uid, gid, err := parseUserGroup(platform.UserGroup())
		require.NoError(t, err)
		for _, name := range []string{proxy.CACertFile, proxy.CAKeyFile, proxyCredentialsFile} {
			entry := proxyCredentialsSubdir + "/" + name
			require.Contains(t, files, entry)
			assert.Equal(t, int64(0o600), files[entry].header.Mode, name)
			assert.Equal(t, uid, files[entry].header.Uid, name)
			assert.Equal(t, gid, files[entry].header.Gid, name)
		}

		var loaded []proxy.Credential
		require.NoError(t, json.Unmarshal(files[proxyCredentialsSubdir+"/"+proxyCredentialsFile].content, &loaded))
		assert.Equal(t, creds, loaded)

		ca, err := certs.LoadCA(files[proxyCredentialsSubdir+"/"+proxy.CACertFile].content, files[proxyCredentialsSubdir+"/"+proxy.CAKeyFile].content)
		require.NoError(t, err)
		assert.True(t, ca.Scoped())
		assert.Equal(t, files[proxyCredentialsSubdir+"/"+proxy.CACertFile].content, handle.caPEM, "the tool must trust the CA the proxy got")
	})

	t.Run("removes the container and its volume when the copy fails", func(t *testing.T) {
		// Arrange
		get := stubProxyDocker(t, "true")
		stubDockerRunStdinCapture(t, fmt.Errorf("stub: cp failed"))
		rs := RunSpec{Proxy: ProxySpec{Image: "default-proxy", LogDir: t.TempDir(), Credentials: creds}}

		// Act
		_, err := startProxy(rs)

		// Assert
		require.ErrorContains(t, err, "copy proxy credentials")
		assert.NotNil(t, findCall(get(), "rm", "--force", "--volumes"))
		assert.Nil(t, findCall(get(), "start"))
	})

	t.Run("removes the network when its subnets can't be read", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t)
		stdin := stubDockerRunStdinCapture(t, nil)
		rs := RunSpec{Proxy: ProxySpec{Image: "default-proxy", LogDir: t.TempDir(), Credentials: creds}}

		// Act
		_, err := startProxy(rs)

		// Assert
		require.ErrorContains(t, err, "has no subnet")
		assert.NotNil(t, findCall(get(), "network", "rm"))
		assert.Nil(t, findCall(get(), "create"))
		assert.Empty(t, stdin())
	})

	t.Run("removes the proxy when it exits on start", func(t *testing.T) {
		// Arrange
		get := stubProxyDocker(t, "false")
		stubDockerRunStdinCapture(t, nil)
		rs := RunSpec{Proxy: ProxySpec{Image: "default-proxy", LogDir: t.TempDir(), Credentials: creds}}

		// Act
		_, err := startProxy(rs)

		// Assert
		require.ErrorContains(t, err, "proxy exited on start")
		assert.NotNil(t, findCall(get(), "rm", "--force", "--volumes"))
		assert.Nil(t, findCall(get(), "network", "connect"))
	})

	t.Run("without credentials nothing is copied or mounted", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t)
		stdin := stubDockerRunStdinCapture(t, nil)
		rs := RunSpec{Proxy: ProxySpec{Image: "default-proxy", LogDir: t.TempDir()}}

		// Act
		_, err := startProxy(rs)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, stdin())
		createArgs := findCall(get(), "create")
		assert.False(t, hasArgWithPrefix(createArgs, "--env=AGENTIC_PROXY_CA_DIR="))
		assert.False(t, hasArgWithPrefix(createArgs, "--env="+proxy.EnvClientNets+"="), "runs without credentials keep accepting any client")
		assert.NotContains(t, createArgs, "--volume="+proxyRunMountDir)
	})
}

func Test_proxyHandle_checkStarted(t *testing.T) {
	stubProxySettleTime(t, 0)
	creds := []proxy.Credential{{Hosts: []string{"api.example.test"}, Rules: []proxy.InjectRule{{Header: "X-Api-Key", Value: "test-secret"}}}}

	t.Run("running proxy passes", func(t *testing.T) {
		// Arrange
		stubDockerRunBySubcmd(t, map[string]string{"inspect": "true\n"})
		handle := proxyHandle{container: "agentic-proxy-abc", credentials: creds}

		// Act
		err := handle.checkStarted()

		// Assert
		assert.NoError(t, err)
	})

	t.Run("exited proxy fails with its logs", func(t *testing.T) {
		// Arrange
		stubDockerRunBySubcmd(t, map[string]string{"inspect": "false\n", "logs": "load proxy CA: bad key\n"})
		handle := proxyHandle{container: "agentic-proxy-abc", credentials: creds}

		// Act
		err := handle.checkStarted()

		// Assert
		assert.ErrorContains(t, err, "proxy exited on start")
		assert.ErrorContains(t, err, "load proxy CA: bad key")
	})

	t.Run("without credentials nothing is checked", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t)
		handle := proxyHandle{container: "agentic-proxy-abc"}

		// Act
		err := handle.checkStarted()

		// Assert
		require.NoError(t, err)
		assert.Empty(t, get())
	})
}

func Test_dryRunProxy(t *testing.T) {
	t.Run("credentials print no secret and touch nothing", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t)
		stdin := stubDockerRunStdinCapture(t, nil)
		creds := []proxy.Credential{{Hosts: []string{"api.example.test"}, Rules: []proxy.InjectRule{{Header: "X-Api-Key", Value: "test-secret"}}}}
		rs := RunSpec{DryRun: true, Proxy: ProxySpec{Mode: ProxyEnforce, Image: "default-proxy", LogDir: t.TempDir(), Credentials: creds}}

		// Act
		var toolArgs []string
		out := captureStdout(t, func() {
			var err error
			toolArgs, _, err = dryRunProxy(&rs)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, toolArgs, "--env="+tools.ProxyCAEnvName+"="+dryRunCAPlaceholder)
		assert.Contains(t, out, "--volume="+proxyRunMountDir)
		assert.Contains(t, out, proxy.EnvClientNets+"="+dryRunClientNetsPlaceholder)
		assert.NotContains(t, out, "test-secret")
		assert.Empty(t, calls())
		assert.Empty(t, stdin())
	})
}
