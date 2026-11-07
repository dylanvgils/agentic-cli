//go:build integration

package proxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/network/certs"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/require"
)

// Upstream aliases on the test network; all resolve to the one fake upstream.
const (
	allowedHost    = "upstream.service.test"
	deniedHost     = "denied.service.test"
	credentialHost = "secret.service.test"
	pathHost       = "paths.service.test"
)

const (
	proxyAlias = "proxy.service.test"

	testSecret      = "test-secret"
	testPlaceholder = "agentic-0123456789abcdef"
	testToken       = "test-token"

	// Container paths of the mounted test files.
	certsDir       = "/certs"
	upstreamCA     = "/upstream-ca.pem"
	credentialsDir = "/run/agentic-proxy/credentials"
	logFile        = "/logs/proxy.jsonl"
)

// serviceEnv is a test network with the fake upstream, whose CA the proxy and clients trust.
type serviceEnv struct {
	t          *testing.T
	network    string
	dir        string // mounted at certsDir, holds proxy-ca.pem
	upstreamCA string // host path, mounted at upstreamCA
}

// proxyRun builds one standalone proxy container on a serviceEnv.
type proxyRun struct {
	t          *testing.T
	env        *serviceEnv
	rules      []proxy.Rule
	creds      []proxy.Credential
	monitor    bool
	token      string
	credsDir   string
	noAddr     bool
	unscopedCA bool
	logPath    string
	container  *platform.FakeResource
}

// newServiceEnv creates the network, the certs and the fake upstream serving every upstream alias.
func newServiceEnv(t *testing.T) *serviceEnv {
	t.Helper()
	network := platform.FakeNetwork(t).Create().Name()
	upstream := platform.FakeContainer(t).
		OnNetwork(network).
		WithAlias(allowedHost).
		WithAlias(deniedHost).
		WithAlias(credentialHost).
		WithAlias(pathHost).
		WithEnv("EXPECT_SECRET", "Bearer "+testSecret).
		ServingUpstream().
		Create()

	return &serviceEnv{
		t:          t,
		network:    network,
		dir:        platform.TempDirIn(t, platform.RootDir()),
		upstreamCA: upstream.UpstreamCA(),
	}
}

// curl requests url through the proxy from a client on the network, with a final status=<code> line.
func (e *serviceEnv) curl(url string, args ...string) string {
	e.t.Helper()
	cmd := []string{"curl", "-sS", "-m", "10", "--noproxy", "", "-x", "http://" + proxyAlias + ":" + tools.ProxyPort, "-w", "\nstatus=%{http_code}"}
	cmd = append(cmd, args...)
	return e.client(append(cmd, url)...)
}

// shell runs script in a client on the network, with the certs mounted; it fails the test only if the client can't run.
func (e *serviceEnv) shell(script string) string {
	e.t.Helper()
	return e.client("sh", "-c", script)
}

// client runs cmd in a ToolImage container on the network, with the certs mounted.
func (e *serviceEnv) client(cmd ...string) string {
	e.t.Helper()
	args := []string{
		"run", "--rm",
		"--network", e.network,
		"--user", hostUser(),
		"--volume", e.dir + ":" + certsDir + ":ro",
		"--volume", e.upstreamCA + ":" + upstreamCA + ":ro",
		"--entrypoint", "",
		platform.ToolImage,
	}
	out := platform.Docker(e.t, append(args, cmd...)...)
	e.t.Logf("client %s\n%s", strings.Join(cmd, " "), out)
	return out
}

// subnet returns the network's first subnet, for the proxy's client nets.
func (e *serviceEnv) subnet() string {
	e.t.Helper()
	return platform.Docker(e.t, "network", "inspect", "--format", "{{(index .IPAM.Config 0).Subnet}}", e.network)
}

// newProxyRun starts building a proxy on env.
func newProxyRun(t *testing.T, env *serviceEnv) *proxyRun {
	return &proxyRun{t: t, env: env}
}

// allowing allows hosts whole.
func (r *proxyRun) allowing(hosts ...string) *proxyRun {
	for _, host := range hosts {
		r.rules = append(r.rules, proxy.Rule{Host: host})
	}
	return r
}

// allowingPaths allows host only on paths.
func (r *proxyRun) allowingPaths(host string, paths ...string) *proxyRun {
	r.rules = append(r.rules, proxy.Rule{Host: host, Paths: paths})
	return r
}

// withCredential injects testSecret for credentialHost's Authorization header.
func (r *proxyRun) withCredential() *proxyRun {
	r.creds = append(r.creds, proxy.Credential{
		Hosts:       []string{credentialHost},
		Headers:     []string{"Authorization"},
		Placeholder: testPlaceholder,
		Secret:      testSecret,
	})
	return r
}

// monitoring logs verdicts without enforcing the allowlist.
func (r *proxyRun) monitoring() *proxyRun {
	r.monitor = true
	return r
}

// withToken requires testToken from every client.
func (r *proxyRun) withToken() *proxyRun {
	r.token = testToken
	return r
}

// withoutAddr leaves the listen address unset.
func (r *proxyRun) withoutAddr() *proxyRun {
	r.noAddr = true
	return r
}

// withUnscopedCA hands the proxy a CA without name constraints.
func (r *proxyRun) withUnscopedCA() *proxyRun {
	r.unscopedCA = true
	return r
}

// start creates the proxy and waits until it accepts connections.
func (r *proxyRun) start() *proxyRun {
	r.t.Helper()
	r.create()
	require.Eventually(r.t, func() bool {
		return platform.DockerOK("run", "--rm", "--network", r.env.network, "--entrypoint", "", platform.ToolImage,
			"nc", "-z", proxyAlias, tools.ProxyPort)
	}, 30*time.Second, 500*time.Millisecond, "proxy never started listening")
	return r
}

// create starts the hardened proxy container, as agentic runs it, without waiting for it.
func (r *proxyRun) create() *proxyRun {
	r.t.Helper()
	dir := platform.TempDirIn(r.t, platform.RootDir())
	r.logPath = filepath.Join(dir, "proxy.jsonl")
	writeFile(r.t, r.logPath, nil)

	rules, err := json.Marshal(r.rules)
	require.NoError(r.t, err)

	c := platform.FakeContainer(r.t).
		FromImage(tools.ProxyImage).
		OnNetwork(r.env.network).
		WithAlias(proxyAlias).
		Hardened().
		RunningAs(hostUser()).
		WithEnv(proxy.EnvRules, string(rules)).
		WithEnv(proxy.EnvLog, logFile).
		WithEnv(proxy.EnvMonitor, strconv.FormatBool(r.monitor)).
		WithEnv("SSL_CERT_FILE", upstreamCA).
		WithMount(r.logPath, logFile).
		WithMount(r.env.upstreamCA, upstreamCA+":ro")
	if !r.noAddr {
		c.WithEnv(proxy.EnvAddr, ":"+tools.ProxyPort)
	}
	if r.token != "" {
		c.WithEnv(proxy.EnvAuthToken, r.token)
	}
	if r.decrypts() {
		c.WithEnv(proxy.EnvCADir, credentialsDir).
			WithEnv(proxy.EnvCredentials, credentialsDir+"/credentials.json").
			WithEnv(proxy.EnvClientNets, r.env.subnet()).
			WithMount(r.writeCredentials(), credentialsDir)
	}

	r.container = c.Started().Create()
	r.t.Cleanup(func() {
		if r.t.Failed() {
			r.t.Logf("proxy %s logs\n%s", r.container.Name(), r.output())
		}
	})
	return r
}

// exitOutput waits for the proxy to exit and returns its exit code and output.
func (r *proxyRun) exitOutput() (code, out string) {
	r.t.Helper()
	code = platform.Docker(r.t, "wait", r.container.Name())
	return code, platform.Docker(r.t, "logs", r.container.Name())
}

// output returns what the proxy printed so far.
func (r *proxyRun) output() string {
	r.t.Helper()
	return platform.Docker(r.t, "logs", r.container.Name())
}

// log returns the proxy's access log entries, without the time and error, which vary per run.
func (r *proxyRun) log() []proxy.Entry {
	r.t.Helper()
	f, err := os.Open(r.logPath)
	require.NoError(r.t, err)
	defer func() { _ = f.Close() }()

	var entries []proxy.Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry proxy.Entry
		require.NoError(r.t, json.Unmarshal(scanner.Bytes(), &entry), "proxy log line: %s", scanner.Text())
		entry.Time = time.Time{}
		entry.Error = ""
		entries = append(entries, entry)
	}
	require.NoError(r.t, scanner.Err())
	return entries
}

// decrypts reports whether the proxy needs a CA: for credentials or path rules.
func (r *proxyRun) decrypts() bool {
	return len(r.creds) > 0 || len(proxy.PathHosts(r.rules)) > 0
}

// writeCredentials writes the proxy CA and credentials, returning their dir; clients get the CA as proxy-ca.pem.
func (r *proxyRun) writeCredentials() string {
	r.t.Helper()
	ca, err := certs.NewScopedCA("agentic-itest proxy", append(proxy.CredentialHosts(r.creds), proxy.PathHosts(r.rules)...))
	if r.unscopedCA {
		ca, err = certs.NewCA("agentic-itest proxy")
	}
	require.NoError(r.t, err)

	creds, err := json.Marshal(r.creds)
	require.NoError(r.t, err)

	r.credsDir = platform.TempDirIn(r.t, platform.RootDir())
	writeKeyPair(r.t, filepath.Join(r.credsDir, proxy.CACertFile), filepath.Join(r.credsDir, proxy.CAKeyFile), ca.KeyPair)
	writeFile(r.t, filepath.Join(r.credsDir, "credentials.json"), creds)
	writeFile(r.t, filepath.Join(r.env.dir, "proxy-ca.pem"), ca.CertPEM())
	return r.credsDir
}

// credentialFileExists reports whether the proxy left name in its credentials dir.
func (r *proxyRun) credentialFileExists(name string) bool {
	_, err := os.Stat(filepath.Join(r.credsDir, name))
	return err == nil
}

// hostUser returns the test process's uid:gid, which owns every mounted file.
func hostUser() string {
	return fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
}

// writeKeyPair writes pair's cert and key as PEM.
func writeKeyPair(t *testing.T, certPath, keyPath string, pair certs.KeyPair) {
	t.Helper()
	key, err := pair.KeyPEM()
	require.NoError(t, err)
	writeFile(t, certPath, pair.CertPEM())
	writeFile(t, keyPath, key)
}

// writeFile writes data to path.
func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, data, 0o600))
}
