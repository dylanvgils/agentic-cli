package docker

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/tools"
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

// Startup check settings for a credentialed proxy; vars so tests can shorten them.
var (
	proxySettleTime   = 500 * time.Millisecond
	proxyPollInterval = 50 * time.Millisecond
)

// proxyHandle identifies the per-run proxy sidecar container, host-side access log, injected credentials, the run
// network's subnets that may use them, and the CA cert the proxy signs them with, set once started.
type proxyHandle struct {
	client      *Client
	id          string
	container   string
	logPath     string
	allow       []string
	monitor     bool
	credentials []proxy.Credential
	// placeholderEnv sets each credential's env vars to its placeholder in the tool
	placeholderEnv []string
	clientNets     []string
	caPEM          []byte
}

// newProxyHandle derives the per-run proxy resource names and credential placeholders without creating any docker
// resources, so it is safe for dry runs.
func (c *Client) newProxyHandle(rs toolRun) (proxyHandle, error) {
	id, err := randID()
	if err != nil {
		return proxyHandle{}, err
	}

	creds, placeholderEnv, err := proxyCredentials(rs.Credentials)
	if err != nil {
		return proxyHandle{}, err
	}

	name := proxyHostAlias + "-" + id
	return proxyHandle{
		client:         c,
		id:             id,
		container:      name,
		logPath:        filepath.Join(rs.Proxy.LogDir, proxyLogFileName(id)),
		allow:          rs.Proxy.Allow,
		monitor:        rs.Proxy.Mode == sandbox.ProxyMonitor,
		credentials:    creds,
		placeholderEnv: placeholderEnv,
	}, nil
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

// PrintSummary reports hosts the allowlist blocked, or would have blocked in monitor mode, separately hosts the address
// guard refused, which it does in both modes, and credential hosts the tool reached without the placeholder.
func (h proxyHandle) PrintSummary(l *logging.Logger) {
	hosts, denied := h.hostsMatching(deniedByAllowlist)
	refusedHosts, refused := h.hostsMatching(refusedByGuard)
	missedHosts, missed := h.hostsMatching(missedPlaceholder)
	if denied == 0 && refused == 0 && missed == 0 {
		return
	}

	// Blank line separates the summary from the tool output above it
	fmt.Fprintln(l.Writer())

	switch {
	case denied == 0:
	case h.monitor:
		l.Infof("proxy (monitor mode) observed %d request(s); %d would be blocked under the current allowlist: %s", h.totalRequests(), denied, strings.Join(hosts, ", "))
		l.Detail("add the ones you want to allow to [run.proxy] allowed_hosts, then drop --proxy-monitor.")
	default:
		l.Infof("proxy blocked %d request(s) to: %s", denied, strings.Join(hosts, ", "))
		l.Detail("add them to [run.proxy] allowed_hosts (or pass --no-proxy) to permit.")
	}

	if refused > 0 {
		l.Infof("proxy refused %d request(s) to local or private addresses: %s", refused, strings.Join(refusedHosts, ", "))
		l.Detail("list an internal host exactly in [run.proxy] allowed_hosts to permit its private address; loopback and link-local stay refused.")
	}

	if missed > 0 {
		l.Warnf("proxy saw %d connection(s) to credential hosts without the placeholder, so they likely ran unauthenticated: %s", missed, strings.Join(missedHosts, ", "))
		l.Detail("check that the tool reads one of the credential's env names and sends it in a listed header.")
	}
}

// hostsMatching reads the access log and returns the unique hosts of entries matching match (first-seen order) and their total.
func (h proxyHandle) hostsMatching(match func(proxy.Entry) bool) (hosts []string, total int) {
	f, err := os.Open(h.logPath)
	if err != nil {
		return nil, 0
	}
	defer func() { _ = f.Close() }()

	seen := make(map[string]bool)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry proxy.Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if !match(entry) {
			continue
		}

		total++
		if !seen[entry.Host] {
			seen[entry.Host] = true
			hosts = append(hosts, proxy.DisplayHost(entry.Host))
		}
	}

	// A read error here only means the summary reflects whatever was
	// scanned before it occurred; PrintSummary has no error path to report it.
	_ = scanner.Err()

	return hosts, total
}

// totalRequests reads the access log and returns the total connection attempts logged, regardless of decision; entries
// logged from inside an intercepted tunnel repeat its attempt, so they aren't counted.
func (h proxyHandle) totalRequests() int {
	f, err := os.Open(h.logPath)
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()

	total := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry proxy.Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil || repeatsAttempt(entry) {
			continue
		}
		total++
	}
	_ = scanner.Err()

	return total
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

	return handle.toolEnvArgs(rs.Dind.Enabled), noCleanup, nil
}

// launchProxy starts the sidecar and returns a cleanup that tears it down and prints the access summary.
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
		// Stop before reading the log so late denials make the summary
		handle.Stop()
		stop()
		handle.PrintSummary(logging.Err)
	}
	return handle.toolEnvArgs(rs.Dind.Enabled), cleanup, nil
}

// createArgs builds the `docker create` arguments for the hardened proxy sidecar, registering
// proxyHostAlias on the run network so tool config can reference a stable hostname.
func (h proxyHandle) createArgs(rs toolRun) []string {
	containerLog := proxyLogMountDir + "/" + proxyLogFileName(h.id)
	_, tzOffset := time.Now().Zone()

	args := []string{
		// No --rm, so a proxy that exits on start keeps its logs for checkStarted
		"create", "--read-only",
		arg("name", h.container),
		arg("network", rs.network),
		arg("network-alias", proxyHostAlias),
		arg("cap-drop", "ALL"),
		arg("security-opt", "no-new-privileges:true"),
		arg("user", platform.UserGroup()),
		label(LabelProject, LabelProjectVal),
	}
	args = append(args, ownerLabels(rs)...)

	clientNets := strings.Join(h.clientNets, ",")
	if clientNets == "" {
		clientNets = dryRunClientNetsPlaceholder
	}

	args = append(args,
		arg("env", proxy.EnvAllow+"="+strings.Join(h.allow, ",")),
		arg("env", proxy.EnvLog+"="+containerLog),
		arg("env", proxy.EnvTZOffset+"="+strconv.Itoa(tzOffset)),
		arg("env", proxy.EnvMonitor+"="+strconv.FormatBool(h.monitor)),
		arg("env", proxy.EnvClientNets+"="+clientNets),
		// Only this run's file, so the proxy can't touch other logs
		arg("volume", h.logPath+":"+containerLog),
	)

	if len(h.credentials) > 0 {
		args = append(args,
			arg("env", proxy.EnvCADir+"="+proxyCredentialsDir),
			arg("env", proxy.EnvCredentials+"="+proxyCredentialsDir+"/"+proxyCredentialsFile),
			// Anonymous, so it is removed with the container; the proxy deletes the secret files once loaded
			arg("volume", proxyRunMountDir),
		)
	}

	return append(args, tools.ProxyImage)
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

	connectArgs := []string{"network", "connect", NetworkName, h.container}
	if _, err := h.client.run(connectArgs...); err != nil {
		return nil, fmt.Errorf("connect proxy to %s: %w", NetworkName, err)
	}
	return caPEM, nil
}

// checkStarted fails if a credentialed proxy exits within proxySettleTime, e.g. on a CA or credentials it can't load;
// only credentials add a startup step that fails on bad input, so other runs skip the wait.
func (h proxyHandle) checkStarted() error {
	if len(h.credentials) == 0 {
		return nil
	}

	deadline := time.Now().Add(proxySettleTime)
	for {
		out, err := h.client.run("inspect", arg("format", "{{.State.Running}}"), h.container)
		if err != nil || strings.TrimSpace(out) != "true" {
			logs, _ := h.client.run("logs", arg("tail", "20"), h.container)
			return fmt.Errorf("proxy exited on start:\n%s", strings.TrimSpace(logs))
		}
		if !time.Now().Before(deadline) {
			return nil
		}
		time.Sleep(proxyPollInterval)
	}
}

// SweepProxyResources idempotently removes leftover per-run proxy containers (with their volume), e.g. from an
// interrupted run, scoped to agentic-managed containers named with proxyHostAlias; active runs keep theirs.
func (c *Client) SweepProxyResources() error {
	listArgs := []string{
		"ps", arg("all"), arg("format", ownedFormat(".Names")),
		labelFilter(LabelProject, LabelProjectVal),
		nameFilter(proxyHostAlias),
	}
	return c.removeUnused(listArgs, []string{"rm", arg("force"), arg("volumes")})
}

// proxyLogFileName is shared by the host logPath and the in-container AGENTIC_PROXY_LOG path so they name the same file.
func proxyLogFileName(id string) string {
	return proxy.LogFilePrefix + id + ".jsonl"
}

// randID returns a short random hex identifier for per-run resource names.
func randID() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate proxy id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// deniedByAllowlist reports whether entry was denied because its host isn't on the allowlist, even if monitor mode then let it
// through to an upstream that failed.
func deniedByAllowlist(entry proxy.Entry) bool {
	return entry.Decision == proxy.DecisionDeny && entry.Reason != proxy.ReasonBlockedAddr && entry.Reason != proxy.ReasonNoPlaceholder
}

// missedPlaceholder reports whether entry is a credential tunnel whose requests never carried the placeholder.
func missedPlaceholder(entry proxy.Entry) bool {
	return entry.Reason == proxy.ReasonNoPlaceholder
}

// repeatsAttempt reports whether entry was logged from inside an intercepted tunnel, such as a failed request or a missed
// placeholder, after the tunnel's own entry.
func repeatsAttempt(entry proxy.Entry) bool {
	return entry.Intercepted && entry.Reason != ""
}

// refusedByGuard reports whether the address guard refused entry.
func refusedByGuard(entry proxy.Entry) bool {
	return entry.Reason == proxy.ReasonBlockedAddr
}
