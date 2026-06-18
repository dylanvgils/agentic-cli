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
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
)

// proxyResourcePrefix names the per-run internal network and sidecar container.
const proxyResourcePrefix = "agentic-proxy-"

// proxyLogMountDir is where the host log directory is mounted inside the proxy.
const proxyLogMountDir = "/var/log/agentic-proxy"

// proxyHandle identifies the per-run proxy network, sidecar container, and the
// host-side access log for one run.
type proxyHandle struct {
	id        string
	network   string
	container string
	logPath   string
	allow     []string
}

// newProxyHandle derives the per-run proxy resource names without creating any
// docker resources, so it is also safe to use for dry runs.
func newProxyHandle(rs RunSpec) (proxyHandle, error) {
	id, err := randID()
	if err != nil {
		return proxyHandle{}, err
	}

	name := proxyResourcePrefix + id
	return proxyHandle{
		id:        id,
		network:   name,
		container: name,
		logPath:   filepath.Join(rs.ProxyLogDir, id+".jsonl"),
		allow:     rs.ProxyAllow,
	}, nil
}

// envArgs returns the --env flags that point the tool container at the proxy.
// NO_PROXY excludes loopback only; it is not a security boundary, since the
// internal network already blocks every route except the proxy.
func (h proxyHandle) envArgs() []string {
	url := "http://" + h.container + ":" + proxy.Port
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

// Stop removes the proxy sidecar and its internal network. It is idempotent and
// ignores errors so it is safe to defer.
func (h proxyHandle) Stop() {
	_, _ = dockerRun("rm", "-f", h.container)
	_, _ = dockerRun("network", "rm", h.network)
}

// PrintSummary reports the hosts the proxy blocked during the run, so a user
// whose tool failed to connect knows what to allowlist.
func (h proxyHandle) PrintSummary(w io.Writer) {
	hosts, total := h.deniedHosts()
	if total == 0 {
		return
	}

	fmt.Fprintf(w, "\nagentic proxy blocked %d request(s) to: %s\n", total, strings.Join(hosts, ", "))
	fmt.Fprintln(w, "add them to [run.proxy] allowed_hosts (or pass --no-proxy) to permit.")
}

// deniedHosts reads the access log and returns the unique denied hosts (in
// first-seen order) and the total number of denied requests.
func (h proxyHandle) deniedHosts() (hosts []string, total int) {
	f, err := os.Open(h.logPath)
	if err != nil {
		return nil, 0
	}
	defer f.Close()

	seen := make(map[string]bool)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry proxy.Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.Decision != proxy.DecisionDeny {
			continue
		}

		total++
		if !seen[entry.Host] {
			seen[entry.Host] = true
			hosts = append(hosts, entry.Host)
		}
	}

	return hosts, total
}

// startProxy provisions the per-run internal network and proxy sidecar, wiring
// the sidecar to the egress network so it can reach allowed hosts. On any
// failure it cleans up whatever it created.
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

// runArgs builds the `docker run` arguments for the hardened proxy sidecar.
func (h proxyHandle) runArgs(rs RunSpec) []string {
	containerLog := proxyLogMountDir + "/" + h.id + ".jsonl"

	return []string{
		"run", "--detach", "--rm", "--read-only",
		arg("name", h.container),
		arg("network", h.network),
		arg("cap-drop", "ALL"),
		arg("security-opt", "no-new-privileges:true"),
		arg("user", platform.UserGroup()),
		label(LabelProject, LabelProjectVal),
		arg("env", proxy.EnvAllow+"="+strings.Join(h.allow, ",")),
		arg("env", proxy.EnvLog+"="+containerLog),
		arg("volume", rs.ProxyLogDir+":"+proxyLogMountDir),
		rs.ProxyImage,
	}
}

// SweepProxyResources removes any leftover per-run proxy containers and internal
// networks (e.g. from an interrupted run). It is idempotent and scoped to
// agentic-managed resources named with the proxy prefix.
func SweepProxyResources() error {
	listContainerArgs := []string{
		"ps", arg("all"), arg("quiet"),
		labelFilter(LabelProject, LabelProjectVal),
		nameFilter(proxyResourcePrefix),
	}
	removeContainerArgs := []string{"rm", arg("force")}
	if err := runIfAny(listContainerArgs, removeContainerArgs); err != nil {
		return err
	}

	listNetworkArgs := []string{
		"network", "ls", arg("quiet"),
		labelFilter(LabelProject, LabelProjectVal),
		nameFilter(proxyResourcePrefix),
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
