package docker

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/network/certs"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartProxy(t *testing.T) {
	client := newTestClient()

	stubProxySettleTime(t, client, 0)

	t.Run("creates, then boots the proxy and keeps its CA", func(t *testing.T) {
		// Arrange
		get := stubProxyDocker(t, client, "true")
		stubDockerRunWithStdinCapture(t, client, nil)
		rs := newProxyRun(t, sandbox.RunSpec{Credentials: testCredentials})

		// Act
		handle, err := client.startProxy(newTestProxyHandle(t, client, rs), rs, nil)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, findCall(get(), "create"))
		assert.NotNil(t, findCall(get(), "network", "connect", NetworkName, handle.container))
		assert.NotEmpty(t, handle.caPEM)
	})

	t.Run("fails when the proxy fails to boot", func(t *testing.T) {
		// Arrange
		stubProxyDocker(t, client, "true")
		stubDockerRunWithStdinCapture(t, client, fmt.Errorf("stub: cp failed"))
		rs := newProxyRun(t, sandbox.RunSpec{Credentials: testCredentials})

		// Act
		_, err := client.startProxy(newTestProxyHandle(t, client, rs), rs, nil)

		// Assert
		require.ErrorContains(t, err, "copy proxy credentials")
	})

	t.Run("fails when the create fails", func(t *testing.T) {
		// Arrange
		stubProxyDocker(t, client, "true", "create")
		rs := newProxyRun(t, sandbox.RunSpec{})

		// Act
		_, err := client.startProxy(newTestProxyHandle(t, client, rs), rs, nil)

		// Assert
		require.ErrorContains(t, err, "create proxy")
	})

	t.Run("fails when interrupted while starting", func(t *testing.T) {
		// Arrange
		stubProxyDocker(t, client, "true")
		rs := newProxyRun(t, sandbox.RunSpec{})
		interrupt := make(chan os.Signal, 1)
		interrupt <- os.Interrupt

		// Act
		_, err := client.startProxy(newTestProxyHandle(t, client, rs), rs, interrupt)

		// Assert
		require.EqualError(t, err, "interrupted while starting egress proxy")
	})
}

func TestClient_createProxy(t *testing.T) {
	client := newTestClient()

	t.Run("creates a hardened sidecar on the run network without starting it", func(t *testing.T) {
		// Arrange
		get := stubProxyDocker(t, client, "true")
		rs := newProxyRun(t, sandbox.RunSpec{Egress: sandbox.EgressPolicy{Rules: []sandbox.AllowRule{{Host: "api.anthropic.com"}}}})
		rs.container = "agentic-claude-abc"

		// Act
		handle, err := client.createProxy(newTestProxyHandle(t, client, rs), rs)

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
		assert.Contains(t, createArgs, "--env=AGENTIC_PROXY_ADDR=:3128")
		assert.Contains(t, createArgs, `--env=AGENTIC_PROXY_RULES=[{"host":"api.anthropic.com"}]`)
		assert.Contains(t, createArgs, "--env=AGENTIC_PROXY_MONITOR=false")
		assert.Contains(t, createArgs, "--env=AGENTIC_PROXY_LOG="+proxyLogMountDir+"/proxy_abc.jsonl")
		assert.Contains(t, createArgs, "--volume="+handle.logPath+":"+proxyLogMountDir+"/proxy_abc.jsonl", "only the run's own log file is mounted")
		assert.FileExists(t, handle.logPath)
		assert.True(t, hasArgWithPrefix(createArgs, "--env=AGENTIC_PROXY_TZ_OFFSET="))
		assert.Contains(t, createArgs, "--label=agentic.owner=agentic-claude-abc")
		assert.Contains(t, createArgs, "--label=agentic.run="+string(testRunID))
		assert.True(t, hasArgWithPrefix(createArgs, "--label=agentic.started="))
		assert.Equal(t, tools.ProxyImage, createArgs[len(createArgs)-1])
	})

	t.Run("restricts clients to the run network's subnets", func(t *testing.T) {
		// Arrange
		get := stubProxyDocker(t, client, "true")
		rs := newProxyRun(t, sandbox.RunSpec{})

		// Act
		_, err := client.createProxy(newTestProxyHandle(t, client, rs), rs)

		// Assert
		require.NoError(t, err)
		calls := get()
		assert.NotNil(t, findCall(calls, "network", "inspect", "--format={{range .IPAM.Config}}{{.Subnet}} {{end}}", "agentic-run-abc"), "the subnets are the run network's")
		assert.Contains(t, findCall(calls, "create"), "--env="+proxy.EnvClientNets+"=172.30.0.0/16", "other containers on the egress network may not use the proxy")
	})

	t.Run("credentials add their files and an anonymous volume", func(t *testing.T) {
		// Arrange
		get := stubProxyDocker(t, client, "true")
		rs := newProxyRun(t, sandbox.RunSpec{Credentials: testCredentials})

		// Act
		_, err := client.createProxy(newTestProxyHandle(t, client, rs), rs)

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

	t.Run("path rules add their env, the CA files and an anonymous volume", func(t *testing.T) {
		// Arrange
		get := stubProxyDocker(t, client, "true")
		rules := []sandbox.AllowRule{{Host: "raw.example.test", Paths: []string{"/org/repo/**"}}}
		rs := newProxyRun(t, sandbox.RunSpec{Egress: sandbox.EgressPolicy{Rules: rules}})

		// Act
		_, err := client.createProxy(newTestProxyHandle(t, client, rs), rs)

		// Assert
		require.NoError(t, err)
		createArgs := findCall(get(), "create")
		assert.Contains(t, createArgs, `--env=AGENTIC_PROXY_RULES=[{"host":"raw.example.test","paths":["/org/repo/**"]}]`)
		assert.Contains(t, createArgs, "--env=AGENTIC_PROXY_CA_DIR="+proxyCredentialsDir)
		assert.Contains(t, createArgs, "--volume="+proxyRunMountDir)
	})

	t.Run("without credentials nothing is mounted", func(t *testing.T) {
		// Arrange
		get := stubProxyDocker(t, client, "true")
		rs := newProxyRun(t, sandbox.RunSpec{})

		// Act
		_, err := client.createProxy(newTestProxyHandle(t, client, rs), rs)

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
				return sandbox.LabelProjectVal + "\n", nil
			}
			return "", nil
		})
		rs := newProxyRun(t, sandbox.RunSpec{})

		// Act
		_, err := client.createProxy(newTestProxyHandle(t, client, rs), rs)

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
		_, err := client.newProxyHandle(toolRun{RunSpec: sandbox.RunSpec{Egress: sandbox.EgressPolicy{Mode: sandbox.EgressEnforce}}})

		// Assert
		assert.ErrorIs(t, err, errNoHostUser)
	})

	t.Run("logs to the run's proxy log", func(t *testing.T) {
		// Arrange
		rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser}, proxyLog: "/example.test/logs/proxy_abc.jsonl"}

		// Act
		handle, err := client.newProxyHandle(rs)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, rs.proxyLog, handle.logPath)
	})

	for _, tc := range []struct {
		name        string
		mode        sandbox.EgressMode
		wantMonitor bool
	}{
		{name: "enforce mode does not monitor", mode: sandbox.EgressEnforce},
		{name: "monitor mode monitors", mode: sandbox.EgressMonitor, wantMonitor: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Egress: sandbox.EgressPolicy{Mode: tc.mode}}}

			// Act
			handle, err := client.newProxyHandle(rs)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.wantMonitor, handle.monitor)
		})
	}

	t.Run("sets the integrations' env", func(t *testing.T) {
		// Arrange
		rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Egress: sandbox.EgressPolicy{Integrations: []string{"maven"}}}}

		// Act
		handle, err := client.newProxyHandle(rs)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"MAVEN_ARGS=--global-settings /etc/agentic/maven/settings.xml"}, handle.integrationEnv)
	})
}

func Test_proxyHandle_toolEnvArgs(t *testing.T) {
	t.Run("adds the placeholders and integrations' env", func(t *testing.T) {
		// Arrange
		h := proxyHandle{
			placeholderEnv: []string{"EXAMPLE_API_KEY=placeholder"},
			integrationEnv: []string{"MAVEN_ARGS=--example"},
		}

		// Act
		args := h.toolEnvArgs(false)

		// Assert
		assert.Contains(t, args, "--env=HTTPS_PROXY=http://agentic-proxy:3128")
		assert.Contains(t, args, "--env=EXAMPLE_API_KEY=placeholder")
		assert.Contains(t, args, "--env=MAVEN_ARGS=--example")
	})
}

func Test_proxyEnvArgs(t *testing.T) {
	t.Run("points at the proxy, excludes loopback and makes node read the proxy vars", func(t *testing.T) {
		// Act
		args := proxyEnvArgs(false)

		// Assert
		assert.Contains(t, args, "--env=HTTPS_PROXY=http://agentic-proxy:3128")
		assert.Contains(t, args, "--env=HTTP_PROXY=http://agentic-proxy:3128")
		assert.Contains(t, args, "--env=NO_PROXY=localhost,127.0.0.1")
		assert.Contains(t, args, "--env=NODE_USE_ENV_PROXY=1")
	})

	t.Run("dind excludes the docker sidecar", func(t *testing.T) {
		// Act
		args := proxyEnvArgs(true)

		// Assert
		assert.Contains(t, args, "--env=NO_PROXY=localhost,127.0.0.1,agentic-docker")
	})
}

func Test_setupProxy(t *testing.T) {
	client := newTestClient()

	t.Run("dry run makes no docker calls", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t, client)
		rs := newProxyRun(t, sandbox.RunSpec{
			Image:  "agentic-claude",
			DryRun: true,
			Egress: sandbox.EgressPolicy{Mode: sandbox.EgressEnforce},
		})

		// Act
		env, err := client.setupProxy(rs, nil)

		// Assert
		require.NoError(t, err)
		assert.NotEmpty(t, env)
		assert.Empty(t, calls(), "dry run must not invoke docker")
	})

	t.Run("propagates startProxy error", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, client, "network inspect", "network create")
		rs := newProxyRun(t, sandbox.RunSpec{Image: "agentic-claude", Egress: sandbox.EgressPolicy{Mode: sandbox.EgressEnforce}})

		// Act
		env, err := client.setupProxy(rs, nil)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, env)
	})
}

func Test_proxyHandle_boot(t *testing.T) {
	client := newTestClient()

	stubProxySettleTime(t, client, 0)
	rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Credentials: testCredentials}}

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

	t.Run("path rules without credentials copy a CA scoped to their hosts", func(t *testing.T) {
		// Arrange
		stubProxyDocker(t, client, "true")
		stdin := stubDockerRunWithStdinCapture(t, client, nil)
		rules := []sandbox.AllowRule{{Host: "raw.example.test", Paths: []string{"/org/repo/**"}}}
		handle, err := client.newProxyHandle(toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Egress: sandbox.EgressPolicy{Rules: rules}}})
		require.NoError(t, err)

		// Act
		caPEM, err := handle.boot()

		// Assert
		require.NoError(t, err)
		require.Len(t, stdin(), 1)
		block, _ := pem.Decode(caPEM)
		require.NotNil(t, block)
		cert, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err)
		assert.Equal(t, []string{"raw.example.test"}, cert.PermittedDNSDomains)
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

	stubProxySettleTime(t, client, 0)
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
		rs := newProxyRun(t, sandbox.RunSpec{
			DryRun:      true,
			Credentials: testCredentials,
			Egress:      sandbox.EgressPolicy{Mode: sandbox.EgressEnforce},
		})

		// Act
		var toolArgs []string
		out := iostreamstest.CaptureOut(t, func() {
			var err error
			toolArgs, err = client.dryRunProxy(rs)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, toolArgs, "--env="+tools.ProxyCAEnvName+"="+dryRunCAPlaceholder)
		assert.Regexp(t, `--env=EXAMPLE_API_KEY=agentic-[0-9a-f]{24}`, strings.Join(toolArgs, " "))
		assert.Contains(t, out, "--volume="+proxyRunMountDir)
		assert.Contains(t, out, proxy.EnvClientNets+"="+dryRunClientNetsPlaceholder)
		_, connect, _ := strings.Cut(out, "docker network")
		assert.Contains(t, connect, "connect")
		assert.Contains(t, connect, NetworkName)
		assert.NotContains(t, connect, "docker create", "the proxy joins the egress network after it is created")
		assert.NotContains(t, out, "test-secret")
		assert.Empty(t, calls())
		assert.Empty(t, stdin())
	})
}

func Test_proxyRules(t *testing.T) {
	t.Run("converts the rules for the proxy", func(t *testing.T) {
		// Arrange
		rules := []sandbox.AllowRule{{Host: "raw.example.test", Paths: []string{"/org/repo/**"}}}

		// Act
		converted, err := proxyRules(rules)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []proxy.Rule{{Host: "raw.example.test", Paths: []string{"/org/repo/**"}}}, converted)
	})

	t.Run("an invalid rule is an error", func(t *testing.T) {
		// Arrange
		rules := []sandbox.AllowRule{{Host: "raw.example.test", Paths: []string{"/org/../repo"}}}

		// Act
		_, err := proxyRules(rules)

		// Assert
		assert.ErrorContains(t, err, "proxy allow rules")
	})
}

func TestSweepProxyResources(t *testing.T) {
	// Arrange
	client := newTestClient()
	get := stubUnusedSweep(t, client, "", ownedRow("agentic-proxy-abc", "agentic-claude-gone", time.Hour))

	// Act
	removed, err := client.SweepProxyResources()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"agentic-proxy-abc"}, removed)
	calls := get()
	assert.Contains(t, calls[1].args, "--filter=name=agentic-proxy")
	assert.Equal(t, []string{"rm", "--force", "--volumes", "agentic-proxy-abc"}, findCall(calls, "rm"))
}
