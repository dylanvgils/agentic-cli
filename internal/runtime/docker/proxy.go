package docker

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/infra/platform"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

const (
	// proxyURL is the proxy's address for the tool's proxy settings.
	proxyURL = "http://" + tools.ProxyHostAlias + ":" + tools.ProxyPort // NOSONAR - proxy on the internal network; HTTPS stays TLS via CONNECT

	// proxyRunMountDir is the proxy's anonymous volume holding the CA and credentials, copied in before it starts.
	proxyRunMountDir = "/run/agentic-proxy"

	// proxyCredentialsSubdir is the proxy-owned dir in proxyRunMountDir holding the files, so the proxy can delete them once loaded.
	proxyCredentialsSubdir = "credentials"

	// proxyCredentialsDir is proxyCredentialsSubdir inside the proxy container.
	proxyCredentialsDir = proxyRunMountDir + "/" + proxyCredentialsSubdir

	// proxyCredentialsFile holds the credential list in proxyCredentialsDir.
	proxyCredentialsFile = "credentials.json"

	// proxyLogMountDir is where the run's log file is mounted inside the proxy.
	proxyLogMountDir = "/var/log/agentic-proxy"

	// dryRunClientNetsPlaceholder stands in for the run network's subnets in dry-run output, as the network isn't created.
	dryRunClientNetsPlaceholder = "<proxy network subnets>"

	// dryRunTokenFilePlaceholder stands in for a published proxy's token env file in dry-run output, as it isn't written.
	dryRunTokenFilePlaceholder = "<auth token file>"
)

// proxyHandle identifies the per-run proxy sidecar container, host-side access log, injected credentials, the run
// network's subnets that may use them, and the CA cert the proxy signs them with, set once started.
type proxyHandle struct {
	client    *Client
	id        string
	container string
	// owner is the tool container, or a pidOwner for a published proxy
	owner       string
	logPath     string
	monitor     bool
	credentials []proxy.Credential
	rules       []proxy.Rule
	// placeholderEnv sets each credential's env vars to its placeholder in the tool
	placeholderEnv []string
	// integrationEnv is the proxy integrations' env
	integrationEnv []string
	clientNets     []string
	// token is the auth token clients of a published proxy send; empty for a sidecar on the run network
	token string
	caPEM []byte
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

	rules, err := proxyRules(rs.Egress.Rules)
	if err != nil {
		return proxyHandle{}, err
	}

	name := tools.ProxyHostAlias + "-" + id
	return proxyHandle{
		client:         c,
		id:             id,
		container:      name,
		owner:          rs.container,
		logPath:        rs.proxyLog,
		monitor:        rs.Egress.Mode == sandbox.EgressMonitor,
		credentials:    creds,
		rules:          rules,
		placeholderEnv: placeholderEnv,
		integrationEnv: proxyIntegrationToolEnv(rs.Egress.Integrations),
		user:           *rs.User,
	}, nil
}

// published reports whether the proxy is published on a host port for a runtime outside Docker, instead of joining a run network.
func (h proxyHandle) published() bool {
	return h.token != ""
}

// decrypts reports whether the proxy terminates TLS, for credentials or path rules, and so needs a CA.
func (h proxyHandle) decrypts() bool {
	return len(h.credentials) > 0 || len(proxy.PathHosts(h.rules)) > 0
}

// toolEnvArgs returns the --env flags the tool needs with this proxy: its URL, CA trust, placeholders and integrations.
func (h proxyHandle) toolEnvArgs(dind bool) []string {
	args := append(proxyEnvArgs(dind), h.trustArgs()...)
	for _, entry := range slices.Concat(h.placeholderEnv, h.integrationEnv) {
		args = append(args, arg("env", entry))
	}
	return args
}

// proxyEnvArgs returns the --env flags pointing the tool at the proxy, keyed on the static
// tools.ProxyHostAlias so the URL is stable despite the container name being randomized per run.
// NO_PROXY excludes loopback (and the dind sidecar) only - not a security boundary; the internal network blocks every other route.
func proxyEnvArgs(dind bool) []string {
	noProxy := "localhost,127.0.0.1"
	if dind {
		// Otherwise the docker CLI would route DOCKER_HOST through the proxy
		noProxy += "," + dindHostAlias
	}
	var args []string
	for _, entry := range sandbox.ProxyEnv(proxyURL, noProxy) {
		args = append(args, arg("env", entry))
	}
	return args
}

// Stop removes the proxy sidecar and its volume; idempotent and error-ignoring, so it is safe to defer.
func (h proxyHandle) Stop() {
	_, _ = h.client.run("rm", "--force", "--volumes", h.container)
}

// startProxy starts h's proxy sidecar, copying in any credentials before it starts and wiring it to the egress
// network; cleans up whatever it created on any failure.
func (c *Client) startProxy(h proxyHandle, rs toolRun) (proxyHandle, error) {
	h, err := c.createProxy(h, rs)
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

// startGuarded starts h's proxy with signals guarded, so Ctrl-C during startup still removes the sidecar and its
// credentials volume; cleanup stops it and the guard.
func (c *Client) startGuarded(h proxyHandle, rs toolRun) (started proxyHandle, cleanup func(), err error) {
	interrupt, stop := guardSignals()

	logging.Info("starting egress proxy...")
	h, err = c.startProxy(h, rs)
	if err != nil {
		stop()
		return proxyHandle{}, nil, err
	}

	select {
	case <-interrupt:
		h.Stop()
		stop()
		return proxyHandle{}, nil, fmt.Errorf("interrupted while starting egress proxy")
	default:
	}

	cleanup = func() {
		h.Stop()
		stop()
	}
	return h, cleanup, nil
}

// createProxy prepares the egress network and log file and creates h's proxy container without starting it.
func (c *Client) createProxy(h proxyHandle, rs toolRun) (proxyHandle, error) {
	if err := c.ensureEgressNetwork(); err != nil {
		return proxyHandle{}, err
	}

	if !h.published() {
		// The proxy also joins the shared egress network, so only this run's network may use it
		var err error
		h.clientNets, err = c.networkSubnets(rs.network)
		if err != nil {
			return proxyHandle{}, err
		}
	}

	if err := h.createLogFile(); err != nil {
		return proxyHandle{}, err
	}

	tokenFile, removeTokenFile, err := h.writeTokenFile()
	if err != nil {
		return proxyHandle{}, err
	}
	defer removeTokenFile()

	if _, err := c.run(h.createArgs(rs, tokenFile)...); err != nil {
		return proxyHandle{}, fmt.Errorf("create proxy: %w", err)
	}
	return h, nil
}

// setupProxy configures rs for proxy mode if enabled, returning the tool args (proxy env, CA trust) to inject and a cleanup func to defer.
func (c *Client) setupProxy(rs toolRun) (proxyEnv []string, cleanup func(), err error) {
	if !rs.Egress.Mode.Enabled() {
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
	sandbox.PrintDryRun("egress proxy", "docker", handle.createArgs(rs, ""))
	sandbox.PrintDryRun("egress network", "docker", handle.connectArgs())

	return handle.toolEnvArgs(rs.Dind.Enabled), noCleanup, nil
}

// launchProxy starts the sidecar and returns a cleanup that tears it down.
func (c *Client) launchProxy(rs toolRun) (proxyEnv []string, cleanup func(), err error) {
	h, err := c.newProxyHandle(rs)
	if err != nil {
		return nil, nil, err
	}

	h, cleanup, err = c.startGuarded(h, rs)
	if err != nil {
		return nil, nil, err
	}
	return h.toolEnvArgs(rs.Dind.Enabled), cleanup, nil
}

// createArgs builds the `docker create` arguments for the hardened proxy sidecar: on the run network it registers
// tools.ProxyHostAlias so tool config can reference a stable hostname, and published it joins the egress network
// directly with its port on host loopback, reading its token from tokenFile so the token never shows in process args.
func (h proxyHandle) createArgs(rs toolRun, tokenFile string) []string {
	containerLog := proxyLogMountDir + "/" + filepath.Base(h.logPath)
	_, tzOffset := time.Now().Zone()

	// No --rm, so a proxy that exits on start keeps its logs for checkStarted
	args := []string{"create", "--read-only", arg("name", h.container)}
	if h.published() {
		args = append(args,
			arg("network", NetworkName),
			arg("publish", "127.0.0.1::"+tools.ProxyPort),
		)
	} else {
		args = append(args,
			arg("network", rs.network),
			// Aliases are per network, so concurrent runs can share it
			arg("network-alias", tools.ProxyHostAlias),
		)
	}
	args = append(args,
		arg("cap-drop", "ALL"),
		arg("security-opt", "no-new-privileges:true"),
		arg("user", h.user.String()),
		label(LabelProject, LabelProjectVal),
	)
	args = append(args, ownerLabels(h.owner)...)

	// Validated plain data, so it always marshals
	rulesJSON, _ := json.Marshal(h.rules)
	args = append(args,
		arg("env", proxy.EnvAddr+"=:"+tools.ProxyPort),
		arg("env", proxy.EnvRules+"="+string(rulesJSON)),
		arg("env", proxy.EnvLog+"="+containerLog),
		arg("env", proxy.EnvTZOffset+"="+strconv.Itoa(tzOffset)),
		arg("env", proxy.EnvMonitor+"="+strconv.FormatBool(h.monitor)),
		// Only this run's file, so the proxy can't touch other logs
		arg("volume", h.logPath+":"+containerLog),
	)

	if h.published() {
		if tokenFile == "" {
			tokenFile = dryRunTokenFilePlaceholder
		}
		args = append(args, arg("env-file", tokenFile))
	} else {
		clientNets := strings.Join(h.clientNets, ",")
		if clientNets == "" {
			clientNets = dryRunClientNetsPlaceholder
		}
		args = append(args, arg("env", proxy.EnvClientNets+"="+clientNets))
	}

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

// boot copies in the credentials, starts the created proxy and connects it to the egress network unless published,
// returning the per-run CA; the caller removes the container on failure.
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

	// A published proxy is on the egress network already
	if h.published() {
		return caPEM, nil
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
		nameFilter(tools.ProxyHostAlias),
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
