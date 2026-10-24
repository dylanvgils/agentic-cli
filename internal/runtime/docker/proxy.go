package docker

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/infra/platform"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// proxyHostAlias is the proxy's stable network alias, registered alongside its randomized
// container name so tools needing a literal host:port can hardcode it; safe to reuse across concurrent runs since aliases are scoped per-network.
const proxyHostAlias = "agentic-proxy"

// proxyRunMountDir is the proxy's anonymous volume holding the CA and credentials, copied in before it starts.
const proxyRunMountDir = "/run/agentic-proxy"

// proxyCredentialsSubdir is the proxy-owned dir in proxyRunMountDir holding the files, so the proxy can delete them once loaded.
const proxyCredentialsSubdir = "credentials"

// proxyCredentialsDir is proxyCredentialsSubdir inside the proxy container.
const proxyCredentialsDir = proxyRunMountDir + "/" + proxyCredentialsSubdir

// proxyCredentialsFile holds the credential list in proxyCredentialsDir.
const proxyCredentialsFile = "credentials.json"

// dryRunClientNetsPlaceholder stands in for the run network's subnets in dry-run output, as the network isn't created.
const dryRunClientNetsPlaceholder = "<proxy network subnets>"

// proxyLogMountDir is where the run's log file is mounted inside the proxy.
const proxyLogMountDir = "/var/log/agentic-proxy"

// proxyHandle identifies the per-run proxy sidecar container, host-side access log, injected credentials, the run
// network's subnets that may use them, and the CA cert the proxy signs them with, set once started.
type proxyHandle struct {
	client      *Client
	id          string
	container   string
	logPath     string
	monitor     bool
	credentials []proxy.Credential
	rules       []proxy.Rule
	// placeholderEnv sets each credential's env vars to its placeholder in the tool
	placeholderEnv []string
	clientNets     []string
	caPEM          []byte
	// user runs the proxy and owns its credential files, the same as the tool
	user platform.User
}

// newProxyHandle derives the per-run proxy resource names and credential placeholders without creating any docker
// resources, so it is safe for dry runs.
func (c *Client) newProxyHandle(rs toolRun) (proxyHandle, error) {
	if rs.User == nil {
		return proxyHandle{}, errNoHostUser
	}

	id, err := randID()
	if err != nil {
		return proxyHandle{}, err
	}

	creds, placeholderEnv, err := proxyCredentials(rs.Credentials)
	if err != nil {
		return proxyHandle{}, err
	}

	rules, err := proxyRules(rs.Proxy.Rules)
	if err != nil {
		return proxyHandle{}, err
	}

	name := proxyHostAlias + "-" + id
	return proxyHandle{
		client:         c,
		id:             id,
		container:      name,
		logPath:        rs.Proxy.LogFile,
		monitor:        rs.Proxy.Mode == sandbox.ProxyMonitor,
		credentials:    creds,
		rules:          rules,
		placeholderEnv: placeholderEnv,
		user:           *rs.User,
	}, nil
}

// decrypts reports whether the proxy terminates TLS, for credentials or path rules, and so needs a CA.
func (h proxyHandle) decrypts() bool {
	return len(h.credentials) > 0 || len(proxy.PathHosts(h.rules)) > 0
}

// toolEnvArgs returns the --env flags the tool needs with this proxy: its URL, trust in its CA and the credential placeholders.
func (h proxyHandle) toolEnvArgs(dind bool) []string {
	args := append(proxyEnvArgs(dind), h.trustArgs()...)
	for _, entry := range h.placeholderEnv {
		args = append(args, arg("env", entry))
	}
	return args
}

// proxyEnvArgs returns the --env flags pointing the tool at the proxy, keyed on the static
// proxyHostAlias so the URL is stable despite the container name being randomized per run.
// NO_PROXY excludes loopback (and the dind sidecar) only - not a security boundary; the internal network blocks every other route.
func proxyEnvArgs(dind bool) []string {
	url := "http://" + proxyHostAlias + ":" + proxy.Port // NOSONAR - proxy on the internal network; HTTPS stays TLS via CONNECT
	noProxy := "localhost,127.0.0.1"
	if dind {
		// Otherwise the docker CLI would route DOCKER_HOST through the proxy
		noProxy += "," + dindHostAlias
	}
	return []string{
		arg("env", "HTTP_PROXY="+url),
		arg("env", "HTTPS_PROXY="+url),
		arg("env", "http_proxy="+url),
		arg("env", "https_proxy="+url),
		arg("env", "NO_PROXY="+noProxy),
		arg("env", "no_proxy="+noProxy),
	}
}

// Stop removes the proxy sidecar and its volume; idempotent and error-ignoring, so it is safe to defer.
func (h proxyHandle) Stop() {
	_, _ = h.client.run("rm", "--force", "--volumes", h.container)
}

// startProxy starts the proxy sidecar on rs's run network, copying in any credentials before it starts and wiring it
// to the egress network; cleans up whatever it created on any failure.
func (c *Client) startProxy(rs toolRun) (proxyHandle, error) {
	h, err := c.createProxy(rs)
	if err != nil {
		return proxyHandle{}, err
	}

	h.caPEM, err = h.boot()
	if err != nil {
		h.Stop()
		return proxyHandle{}, err
	}
	return h, nil
}

// createProxy prepares the egress network and log file and creates the proxy container without starting it.
func (c *Client) createProxy(rs toolRun) (proxyHandle, error) {
	h, err := c.newProxyHandle(rs)
	if err != nil {
		return proxyHandle{}, err
	}

	if err := c.ensureEgressNetwork(); err != nil {
		return proxyHandle{}, err
	}

	// The proxy also joins the shared egress network, so only this run's network may use it
	h.clientNets, err = c.networkSubnets(rs.network)
	if err != nil {
		return proxyHandle{}, err
	}

	if err := h.createLogFile(); err != nil {
		return proxyHandle{}, err
	}

	if _, err := c.run(h.createArgs(rs)...); err != nil {
		return proxyHandle{}, fmt.Errorf("create proxy: %w", err)
	}
	return h, nil
}

// setupProxy configures rs for proxy mode if enabled, returning the tool args (proxy env, CA trust) to inject and a cleanup func to defer.
func (c *Client) setupProxy(rs toolRun) (proxyEnv []string, cleanup func(), err error) {
	if !rs.Proxy.Mode.Enabled() {
		return nil, noCleanup, nil
	}

	if rs.DryRun {
		return c.dryRunProxy(rs)
	}
	return c.launchProxy(rs)
}

// dryRunProxy prints the proxy command and returns the tool's proxy env without provisioning anything.
func (c *Client) dryRunProxy(rs toolRun) (proxyEnv []string, cleanup func(), err error) {
	handle, err := c.newProxyHandle(rs)
	if err != nil {
		return nil, nil, err
	}

	// Shows no secrets: those are only copied in before start
	if err := sandbox.PrintDryRun("egress proxy", "docker", handle.createArgs(rs)); err != nil {
		return nil, nil, err
	}
	if err := sandbox.PrintDryRun("egress network", "docker", handle.connectArgs()); err != nil {
		return nil, nil, err
	}

	return handle.toolEnvArgs(rs.Dind.Enabled), noCleanup, nil
}

// launchProxy starts the sidecar and returns a cleanup that tears it down.
func (c *Client) launchProxy(rs toolRun) (proxyEnv []string, cleanup func(), err error) {
	// Guard first so Ctrl-C during startup still removes the sidecar and its credentials volume
	interrupt, stop := guardSignals()

	logging.Infof("starting egress proxy...")
	handle, err := c.startProxy(rs)
	if err != nil {
		stop()
		return nil, nil, err
	}

	select {
	case <-interrupt:
		handle.Stop()
		stop()
		return nil, nil, fmt.Errorf("interrupted while starting egress proxy")
	default:
	}

	cleanup = func() {
		handle.Stop()
		stop()
	}
	return handle.toolEnvArgs(rs.Dind.Enabled), cleanup, nil
}

// createArgs builds the `docker create` arguments for the hardened proxy sidecar, registering
// proxyHostAlias on the run network so tool config can reference a stable hostname.
func (h proxyHandle) createArgs(rs toolRun) []string {
	containerLog := proxyLogMountDir + "/" + filepath.Base(h.logPath)
	_, tzOffset := time.Now().Zone()

	args := []string{
		// No --rm, so a proxy that exits on start keeps its logs for checkStarted
		"create", "--read-only",
		arg("name", h.container),
		arg("network", rs.network),
		arg("network-alias", proxyHostAlias),
		arg("cap-drop", "ALL"),
		arg("security-opt", "no-new-privileges:true"),
		arg("user", h.user.String()),
		label(LabelProject, LabelProjectVal),
	}
	args = append(args, ownerLabels(rs)...)

	clientNets := strings.Join(h.clientNets, ",")
	if clientNets == "" {
		clientNets = dryRunClientNetsPlaceholder
	}

	// Validated plain data, so it always marshals
	rulesJSON, _ := json.Marshal(h.rules)
	args = append(args,
		arg("env", proxy.EnvRules+"="+string(rulesJSON)),
		arg("env", proxy.EnvLog+"="+containerLog),
		arg("env", proxy.EnvTZOffset+"="+strconv.Itoa(tzOffset)),
		arg("env", proxy.EnvMonitor+"="+strconv.FormatBool(h.monitor)),
		arg("env", proxy.EnvClientNets+"="+clientNets),
		// Only this run's file, so the proxy can't touch other logs
		arg("volume", h.logPath+":"+containerLog),
	)

	if h.decrypts() {
		args = append(args,
			arg("env", proxy.EnvCADir+"="+proxyCredentialsDir),
			arg("env", proxy.EnvCredentials+"="+proxyCredentialsDir+"/"+proxyCredentialsFile),
			// Anonymous, so it is removed with the container; the proxy deletes the secret files once loaded
			arg("volume", proxyRunMountDir),
		)
	}

	return append(args, tools.ProxyImage)
}

// connectArgs builds the `docker network connect` arguments joining the proxy to the shared egress network.
func (h proxyHandle) connectArgs() []string {
	return []string{"network", "connect", NetworkName, h.container}
}

// createLogFile creates the run's empty log file, so Docker bind-mounts a file instead of creating a root-owned dir.
func (h proxyHandle) createLogFile() error {
	f, err := os.OpenFile(h.logPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create proxy log: %w", err)
	}
	return f.Close()
}

// boot copies in the credentials, starts the created proxy and connects it to the egress network, returning the
// per-run CA; the caller removes the container on failure.
func (h proxyHandle) boot() (caPEM []byte, err error) {
	caPEM, err = h.copyCredentials()
	if err != nil {
		return nil, err
	}

	if _, err := h.client.run("start", h.container); err != nil {
		return nil, fmt.Errorf("start proxy: %w", err)
	}
	if err := h.checkStarted(); err != nil {
		return nil, err
	}

	if _, err := h.client.run(h.connectArgs()...); err != nil {
		return nil, fmt.Errorf("connect proxy to %s: %w", NetworkName, err)
	}
	return caPEM, nil
}

// checkStarted fails if a decrypting proxy exits within proxySettleTime, e.g. on a CA, credentials or path rules it can't
// load; only those add a startup step that fails on bad input, so other runs skip the wait.
func (h proxyHandle) checkStarted() error {
	if !h.decrypts() {
		return nil
	}

	deadline := time.Now().Add(h.client.timing.proxySettleTime)
	for {
		out, err := h.client.run("inspect", arg("format", "{{.State.Running}}"), h.container)
		if err != nil || strings.TrimSpace(out) != "true" {
			logs, _ := h.client.run("logs", arg("tail", "20"), h.container)
			return fmt.Errorf("proxy exited on start:\n%s", strings.TrimSpace(logs))
		}
		if !time.Now().Before(deadline) {
			return nil
		}
		time.Sleep(h.client.timing.proxyPollInterval)
	}
}

// SweepProxyResources removes proxy sidecars of interrupted runs and returns their names.
func (c *Client) SweepProxyResources() ([]string, error) {
	listArgs := []string{
		"ps", arg("all"), arg("format", ownedFormat(".Names")),
		labelFilter(LabelProject, LabelProjectVal),
		nameFilter(proxyHostAlias),
	}
	return c.removeUnused(listArgs, []string{"rm", arg("force"), arg("volumes")})
}

// proxyRules returns rules as the proxy's allow rules, validated so a bad one fails before the proxy starts.
func proxyRules(rules []sandbox.AllowRule) ([]proxy.Rule, error) {
	converted := make([]proxy.Rule, 0, len(rules))
	for _, rule := range rules {
		converted = append(converted, proxy.Rule{Host: rule.Host, Paths: rule.Paths})
	}

	if err := proxy.ValidateRules(converted); err != nil {
		return nil, fmt.Errorf("proxy allow rules: %w", err)
	}
	return converted, nil
}

// randID returns a short random hex identifier for per-run resource names.
func randID() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate proxy id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
