package docker

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
)

// proxyHostAlias is the proxy's stable network alias, registered alongside its randomized
// container name so tools needing a literal host:port can hardcode it; safe to reuse across concurrent runs since aliases are scoped per-network.
const proxyHostAlias = "agentic-proxy"

// proxyLogMountDir is where the host log directory is mounted inside the proxy.
const proxyLogMountDir = "/var/log/agentic-proxy"

// proxyHandle identifies the per-run proxy network, sidecar container, and host-side access log.
type proxyHandle struct {
	id        string
	network   string
	container string
	logPath   string
	allow     []string
	monitor   bool
}

// newProxyHandle derives the per-run proxy resource names without creating any docker resources, so it is safe for dry runs.
func newProxyHandle(rs RunSpec) (proxyHandle, error) {
	id, err := randID()
	if err != nil {
		return proxyHandle{}, err
	}

	name := proxyHostAlias + "-" + id
	return proxyHandle{
		id:        id,
		network:   name,
		container: name,
		logPath:   filepath.Join(rs.ProxyLogDir, id+".jsonl"),
		allow:     rs.ProxyAllow,
		monitor:   rs.ProxyMonitor,
	}, nil
}

// proxyEnvArgs returns the --env flags pointing the tool at the proxy, keyed on the static
// proxyHostAlias so the URL is stable despite the container name being randomized per run.
// NO_PROXY excludes loopback only - not a security boundary; the internal network blocks every other route.
func proxyEnvArgs() []string {
	url := "http://" + proxyHostAlias + ":" + proxy.Port
	noProxy := "localhost,127.0.0.1"
	return []string{
		arg("env", "HTTP_PROXY="+url),
		arg("env", "HTTPS_PROXY="+url),
		arg("env", "http_proxy="+url),
		arg("env", "https_proxy="+url),
		arg("env", "NO_PROXY="+noProxy),
		arg("env", "no_proxy="+noProxy),
	}
}

// Stop removes the proxy sidecar and its internal network; idempotent and error-ignoring, so it is safe to defer.
func (h proxyHandle) Stop() {
	_, _ = dockerRun("rm", "-f", h.container)
	_, _ = dockerRun("network", "rm", h.network)
}

// PrintSummary reports hosts actually blocked in normal mode, or hosts that would have been
// blocked under the current allowlist in monitor mode, since nothing is blocked there.
func (h proxyHandle) PrintSummary(w io.Writer) {
	hosts, denied := h.hostsByDecision(proxy.DecisionDeny)
	if denied == 0 {
		return
	}

	if h.monitor {
		fmt.Fprintf(w, "\nagentic proxy (monitor mode) observed %d request(s); %d would be blocked under the current allowlist: %s\n", h.totalRequests(), denied, strings.Join(hosts, ", "))
		fmt.Fprintln(w, "add the ones you want to allow to [run.proxy] allowed_hosts, then drop --proxy-monitor.")
		return
	}

	fmt.Fprintf(w, "\nagentic proxy blocked %d request(s) to: %s\n", denied, strings.Join(hosts, ", "))
	fmt.Fprintln(w, "add them to [run.proxy] allowed_hosts (or pass --no-proxy) to permit.")
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

// startProxy provisions the per-run internal network and proxy sidecar, wiring the sidecar to
// the egress network; cleans up whatever it created on any failure.
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
		h.network,
	}
	if _, err := dockerRun(createArgs...); err != nil {
		return proxyHandle{}, fmt.Errorf("create proxy network: %w", err)
	}

	if _, err := dockerRun(h.runArgs(rs)...); err != nil {
		_, _ = dockerRun("network", "rm", h.network)
		return proxyHandle{}, fmt.Errorf("start proxy: %w", err)
	}

	connectArgs := []string{"network", "connect", NetworkName, h.container}
	if _, err := dockerRun(connectArgs...); err != nil {
		h.Stop()
		return proxyHandle{}, fmt.Errorf("connect proxy to %s: %w", NetworkName, err)
	}

	return h, nil
}

// runArgs builds the `docker run` arguments for the hardened proxy sidecar, registering
// proxyHostAlias on the per-run network so tool config can reference a stable hostname.
func (h proxyHandle) runArgs(rs RunSpec) []string {
	containerLog := proxyLogMountDir + "/" + h.id + ".jsonl"
	_, tzOffset := time.Now().Zone()

	return []string{
		"run", "--detach", "--rm", "--read-only",
		arg("name", h.container),
		arg("network", h.network),
		arg("network-alias", proxyHostAlias),
		arg("cap-drop", "ALL"),
		arg("security-opt", "no-new-privileges:true"),
		arg("user", platform.UserGroup()),
		label(LabelProject, LabelProjectVal),
		arg("env", proxy.EnvAllow+"="+strings.Join(h.allow, ",")),
		arg("env", proxy.EnvLog+"="+containerLog),
		arg("env", proxy.EnvTZOffset+"="+strconv.Itoa(tzOffset)),
		arg("env", proxy.EnvMonitor+"="+strconv.FormatBool(h.monitor)),
		arg("volume", rs.ProxyLogDir+":"+proxyLogMountDir),
		rs.ProxyImage,
	}
}

// SweepProxyResources idempotently removes leftover per-run proxy containers and internal
// networks (e.g. from an interrupted run), scoped to agentic-managed resources named with proxyHostAlias.
func SweepProxyResources() error {
	listContainerArgs := []string{
		"ps", arg("all"), arg("quiet"),
		labelFilter(LabelProject, LabelProjectVal),
		nameFilter(proxyHostAlias),
	}
	removeContainerArgs := []string{"rm", arg("force")}
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

// randID returns a short random hex identifier for per-run resource names.
func randID() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate proxy id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
