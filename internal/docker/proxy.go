package docker

import (
	"archive/tar"
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/certs"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
)

// proxyHostAlias is the proxy's stable network alias, registered alongside its randomized
// container name so tools needing a literal host:port can hardcode it; safe to reuse across concurrent runs since aliases are scoped per-network.
const proxyHostAlias = "agentic-proxy"

// proxyRunMountDir is the proxy's anonymous volume holding the CA and credentials, copied in before it starts.
const proxyRunMountDir = "/run/agentic-proxy"

// proxyCredentialsFile holds the credential list in proxyRunMountDir.
const proxyCredentialsFile = "credentials.json"

// proxyLogMountDir is where the host log directory is mounted inside the proxy.
const proxyLogMountDir = "/var/log/agentic-proxy"

// Proxy modes; ProxyOff is the zero value, so a RunSpec without one runs no proxy.
const (
	ProxyOff ProxyMode = iota
	ProxyEnforce
	ProxyMonitor
)

// Startup check settings for a credentialed proxy; vars so tests can shorten them.
var (
	proxySettleTime   = 500 * time.Millisecond
	proxyPollInterval = 50 * time.Millisecond
)

// ProxyMode selects whether the egress proxy runs and whether it blocks or only logs disallowed hosts.
type ProxyMode int

// ProxySpec configures the egress proxy sidecar; unless Mode is ProxyOff, the tool reaches out only through it.
type ProxySpec struct {
	Mode   ProxyMode
	Image  string   // proxy sidecar image
	Allow  []string // merged allowlist (tool baseline + user hosts)
	LogDir string   // host dir for JSON-lines access logs
	// Credentials are the headers the proxy injects; they hold secrets, so never print them
	Credentials []proxy.Credential
}

// proxyHandle identifies the per-run proxy network, sidecar container, host-side access log and injected credentials.
type proxyHandle struct {
	id          string
	network     string
	container   string
	logPath     string
	allow       []string
	monitor     bool
	credentials []proxy.Credential
}

// Enabled reports whether the proxy runs, in either mode.
func (m ProxyMode) Enabled() bool {
	return m != ProxyOff
}

// newProxyHandle derives the per-run proxy resource names without creating any docker resources, so it is safe for dry runs.
func newProxyHandle(rs RunSpec) (proxyHandle, error) {
	id, err := randID()
	if err != nil {
		return proxyHandle{}, err
	}

	name := proxyHostAlias + "-" + id
	return proxyHandle{
		id:          id,
		network:     name,
		container:   name,
		logPath:     filepath.Join(rs.Proxy.LogDir, proxyLogFileName(id)),
		allow:       rs.Proxy.Allow,
		monitor:     rs.Proxy.Mode == ProxyMonitor,
		credentials: rs.Proxy.Credentials,
	}, nil
}

// proxyEnvArgs returns the --env flags pointing the tool at the proxy, keyed on the static
// proxyHostAlias so the URL is stable despite the container name being randomized per run.
// NO_PROXY excludes loopback (and the dind sidecar) only - not a security boundary; the internal network blocks every other route.
func proxyEnvArgs(dind bool) []string {
	url := "http://" + proxyHostAlias + ":" + proxy.Port
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

// Stop removes the proxy sidecar, its volume and its internal network; idempotent and error-ignoring, so it is safe to defer.
func (h proxyHandle) Stop() {
	_, _ = dockerRun("rm", "--force", "--volumes", h.container)
	_, _ = dockerRun("network", "rm", h.network)
}

// PrintSummary reports hosts actually blocked in normal mode, or hosts that would have been
// blocked under the current allowlist in monitor mode, since nothing is blocked there.
func (h proxyHandle) PrintSummary(l *logging.Logger) {
	hosts, denied := h.hostsByDecision(proxy.DecisionDeny)
	if denied == 0 {
		return
	}

	// Blank line separates the summary from the tool output above it
	fmt.Fprintln(l.Writer())

	if h.monitor {
		l.Infof("proxy (monitor mode) observed %d request(s); %d would be blocked under the current allowlist: %s", h.totalRequests(), denied, strings.Join(hosts, ", "))
		l.Detail("add the ones you want to allow to [run.proxy] allowed_hosts, then drop --proxy-monitor.")
		return
	}

	l.Infof("proxy blocked %d request(s) to: %s", denied, strings.Join(hosts, ", "))
	l.Detail("add them to [run.proxy] allowed_hosts (or pass --no-proxy) to permit.")
}

// hostsByDecision reads the access log and returns the unique hosts logged with decision (first-seen order) and the matching total.
func (h proxyHandle) hostsByDecision(decision proxy.Decision) (hosts []string, total int) {
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
		if entry.Decision != decision {
			continue
		}

		total++
		if !seen[entry.Host] {
			seen[entry.Host] = true
			hosts = append(hosts, entry.Host)
		}
	}

	// A read error here only means the summary reflects whatever was
	// scanned before it occurred; PrintSummary has no error path to report it.
	_ = scanner.Err()

	return hosts, total
}

// totalRequests reads the access log and returns the total connection attempts logged, regardless of decision.
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
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		total++
	}
	_ = scanner.Err()

	return total
}

// startProxy provisions the per-run internal network and proxy sidecar, copying in any credentials before it starts
// and wiring it to the egress network; cleans up whatever it created on any failure.
func startProxy(rs RunSpec) (proxyHandle, error) {
	h, err := newProxyHandle(rs)
	if err != nil {
		return proxyHandle{}, err
	}

	if err := EnsureNetwork(); err != nil {
		return proxyHandle{}, err
	}

	createArgs := []string{
		"network", "create",
		arg("internal"),
		label(LabelProject, LabelProjectVal),
	}
	createArgs = append(createArgs, ownerLabels(rs)...)
	createArgs = append(createArgs, h.network)
	if _, err := dockerRun(createArgs...); err != nil {
		return proxyHandle{}, fmt.Errorf("create proxy network: %w", err)
	}

	if _, err := dockerRun(h.createArgs(rs)...); err != nil {
		_, _ = dockerRun("network", "rm", h.network)
		return proxyHandle{}, fmt.Errorf("create proxy: %w", err)
	}

	if err := h.copyCredentials(); err != nil {
		h.Stop()
		return proxyHandle{}, err
	}

	if _, err := dockerRun("start", h.container); err != nil {
		h.Stop()
		return proxyHandle{}, fmt.Errorf("start proxy: %w", err)
	}
	if err := h.checkStarted(); err != nil {
		h.Stop()
		return proxyHandle{}, err
	}

	connectArgs := []string{"network", "connect", NetworkName, h.container}
	if _, err := dockerRun(connectArgs...); err != nil {
		h.Stop()
		return proxyHandle{}, fmt.Errorf("connect proxy to %s: %w", NetworkName, err)
	}

	return h, nil
}

// setupProxy configures rs for proxy mode if enabled, returning the env args to inject and a cleanup func to defer.
func setupProxy(rs *RunSpec) (proxyEnv []string, cleanup func(), err error) {
	if !rs.Proxy.Mode.Enabled() {
		return nil, func() {}, nil
	}

	if rs.DryRun {
		return dryRunProxy(rs)
	}
	return launchProxy(rs)
}

// dryRunProxy reflects the proxy network and env in the printed command without provisioning anything.
func dryRunProxy(rs *RunSpec) (proxyEnv []string, cleanup func(), err error) {
	handle, err := newProxyHandle(*rs)
	if err != nil {
		return nil, nil, err
	}
	rs.network = handle.network

	// Shows no secrets: those are only copied in before start
	if _, err := fmt.Fprintln(os.Stdout, "docker", shellJoin(handle.createArgs(*rs))); err != nil {
		return nil, nil, err
	}

	return proxyEnvArgs(rs.Dind.Enabled), func() {}, nil
}

// launchProxy starts the sidecar and returns a cleanup that tears it down and prints the access summary.
func launchProxy(rs *RunSpec) (proxyEnv []string, cleanup func(), err error) {
	logging.Infof("starting egress proxy...")
	handle, err := startProxy(*rs)
	if err != nil {
		return nil, nil, err
	}
	rs.network = handle.network

	// Capture Ctrl-C so the deferred cleanup still runs after the tool exits
	_, stop := guardSignals()

	cleanup = func() {
		// Stop before reading the log so late denials make the summary
		handle.Stop()
		stop()
		handle.PrintSummary(logging.Err)
	}
	return proxyEnvArgs(rs.Dind.Enabled), cleanup, nil
}

// createArgs builds the `docker create` arguments for the hardened proxy sidecar, registering
// proxyHostAlias on the per-run network so tool config can reference a stable hostname.
func (h proxyHandle) createArgs(rs RunSpec) []string {
	containerLog := proxyLogMountDir + "/" + proxyLogFileName(h.id)
	_, tzOffset := time.Now().Zone()

	args := []string{
		// No --rm, so a proxy that exits on start keeps its logs for checkStarted
		"create", "--read-only",
		arg("name", h.container),
		arg("network", h.network),
		arg("network-alias", proxyHostAlias),
		arg("cap-drop", "ALL"),
		arg("security-opt", "no-new-privileges:true"),
		arg("user", platform.UserGroup()),
		label(LabelProject, LabelProjectVal),
	}
	args = append(args, ownerLabels(rs)...)

	args = append(args,
		arg("env", proxy.EnvAllow+"="+strings.Join(h.allow, ",")),
		arg("env", proxy.EnvLog+"="+containerLog),
		arg("env", proxy.EnvTZOffset+"="+strconv.Itoa(tzOffset)),
		arg("env", proxy.EnvMonitor+"="+strconv.FormatBool(h.monitor)),
		arg("volume", rs.Proxy.LogDir+":"+proxyLogMountDir),
	)

	if len(h.credentials) > 0 {
		args = append(args,
			arg("env", proxy.EnvCADir+"="+proxyRunMountDir),
			arg("env", proxy.EnvCredentials+"="+proxyRunMountDir+"/"+proxyCredentialsFile),
			// Anonymous, so it is removed with the container and has no host path
			arg("volume", proxyRunMountDir),
		)
	}

	return append(args, rs.Proxy.Image)
}

// copyCredentials streams a CA scoped to the credential hosts and the credential list into the created
// container's volume, so neither touches the host disk; a no-op without credentials.
func (h proxyHandle) copyCredentials() error {
	if len(h.credentials) == 0 {
		return nil
	}

	archive, err := credentialArchive(h.credentials)
	if err != nil {
		return err
	}

	// --archive keeps the tar's owner, so the proxy user can read the 0600 files
	if _, err := dockerRunStdin(bytes.NewReader(archive), "cp", arg("archive"), "-", h.container+":"+proxyRunMountDir); err != nil {
		return fmt.Errorf("copy proxy credentials: %w", err)
	}
	return nil
}

// checkStarted fails if a credentialed proxy exits within proxySettleTime, e.g. on a CA or credentials it can't load;
// only credentials add a startup step that fails on bad input, so other runs skip the wait.
func (h proxyHandle) checkStarted() error {
	if len(h.credentials) == 0 {
		return nil
	}

	deadline := time.Now().Add(proxySettleTime)
	for {
		out, err := dockerRun("inspect", arg("format", "{{.State.Running}}"), h.container)
		if err != nil || strings.TrimSpace(out) != "true" {
			logs, _ := dockerRun("logs", arg("tail", "20"), h.container)
			return fmt.Errorf("proxy exited on start:\n%s", strings.TrimSpace(logs))
		}
		if !time.Now().Before(deadline) {
			return nil
		}
		time.Sleep(proxyPollInterval)
	}
}

// SweepProxyResources idempotently removes leftover per-run proxy containers (with their volume) and internal
// networks (e.g. from an interrupted run), scoped to agentic-managed resources named with proxyHostAlias.
func SweepProxyResources() error {
	listContainerArgs := []string{
		"ps", arg("all"), arg("quiet"),
		labelFilter(LabelProject, LabelProjectVal),
		nameFilter(proxyHostAlias),
	}
	removeContainerArgs := []string{"rm", arg("force"), arg("volumes")}
	if err := runIfAny(listContainerArgs, removeContainerArgs); err != nil {
		return err
	}

	listNetworkArgs := []string{
		"network", "ls", arg("quiet"),
		labelFilter(LabelProject, LabelProjectVal),
		nameFilter(proxyHostAlias),
	}
	removeNetworkArgs := []string{"network", "rm"}
	return runIfAny(listNetworkArgs, removeNetworkArgs)
}

// credentialArchive returns a tar of the proxy CA, its key and creds, as 0600 files owned by the host user.
func credentialArchive(creds []proxy.Credential) ([]byte, error) {
	ca, err := certs.NewScopedCA("agentic proxy CA", proxy.CredentialHosts(creds))
	if err != nil {
		return nil, err
	}
	keyPEM, err := ca.KeyPEM()
	if err != nil {
		return nil, err
	}
	credsJSON, err := json.Marshal(creds)
	if err != nil {
		return nil, fmt.Errorf("encode proxy credentials: %w", err)
	}

	uid, gid, err := parseUserGroup(platform.UserGroup())
	if err != nil {
		return nil, err
	}

	files := []struct {
		name    string
		content []byte
	}{
		{proxy.CACertFile, ca.CertPEM()},
		{proxy.CAKeyFile, keyPEM},
		{proxyCredentialsFile, credsJSON},
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, file := range files {
		header := &tar.Header{
			Name:    file.name,
			Mode:    0o600,
			Size:    int64(len(file.content)),
			Uid:     uid,
			Gid:     gid,
			ModTime: time.Now(),
		}
		if err := tw.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("archive proxy %s: %w", file.name, err)
		}
		if _, err := tw.Write(file.content); err != nil {
			return nil, fmt.Errorf("archive proxy %s: %w", file.name, err)
		}
	}
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("archive proxy credentials: %w", err)
	}

	return buf.Bytes(), nil
}

// parseUserGroup splits a "uid:gid" string as returned by platform.UserGroup.
func parseUserGroup(userGroup string) (uid, gid int, err error) {
	u, g, ok := strings.Cut(userGroup, ":")
	if !ok {
		return 0, 0, fmt.Errorf("invalid user %q: expected uid:gid", userGroup)
	}

	uid, err = strconv.Atoi(u)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid uid in %q: %w", userGroup, err)
	}
	gid, err = strconv.Atoi(g)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid gid in %q: %w", userGroup, err)
	}
	return uid, gid, nil
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
