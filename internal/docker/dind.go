package docker

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/dind"
	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
)

const (
	// dindHostAlias is the sidecar's network alias, so DOCKER_HOST is stable across runs.
	dindHostAlias = "agentic-docker"

	// dindNetworkPrefix names the per-run network used when there's no proxy network to join.
	dindNetworkPrefix = "agentic-dind"

	// dindPort is dockerd's TLS port.
	dindPort = "2376"

	// dindDirName is the subdirectory under the agentic home holding per-run dirs.
	dindDirName = "dind"

	// dindCertMountDir is the sidecar's DOCKER_TLS_CERTDIR.
	dindCertMountDir = "/certs"

	// dindClientCertPath is where the tool container sees its client cert.
	dindClientCertPath = "/run/secrets/docker"

	// dindEtcSubdir holds the generated /etc identity files.
	dindEtcSubdir = "etc"

	// dindConfigSubdir holds the tool's writable DOCKER_CONFIG, mounted at dindConfigPath.
	dindConfigSubdir = "config"
	dindConfigPath   = "/run/docker-config"

	// dindDataRoot is a subdir of the image's VOLUME that dockerd creates, so it owns it regardless of host uid.
	dindDataRoot = dind.Home + "/.local/share/docker/data"

	// dindProbeTimeout bounds each readiness probe in seconds, so a hung dockerd can't stall it.
	dindProbeTimeout = "10"
)

// hostUserGroup is a test-stubbable indirection into platform.UserGroup.
var hostUserGroup = platform.UserGroup

// Readiness wait settings; vars so tests can shorten them.
var (
	dindReadyTimeout     = 90 * time.Second
	dindPollInterval     = time.Second
	dindProgressInterval = 15 * time.Second
	dindLogTailOnFail    = "30"
)

// dindRunDirGrace keeps a concurrent run's not-yet-started run dir from being swept.
var dindRunDirGrace = 10 * time.Minute

// dindRunIDPattern matches randID output, so the sweep only touches dirs agentic created.
var dindRunIDPattern = regexp.MustCompile(`^[0-9a-f]{12}$`)

// dindCapabilities is the sidecar's bounding set; they only take effect inside rootlesskit's user namespace.
// A cap missing here can never be regained by dockerd or its containers, even inside a nested user namespace.
var dindCapabilities = []string{
	// rootlesskit: create the user/mount namespaces, mount procfs and overlayfs, pivot_root for runc
	"SYS_ADMIN",
	// rootlesskit/dockerd: set up the network namespace, tap device, bridge and iptables rules
	"NET_ADMIN",
	// newuidmap/newgidmap: write the subuid/subgid maps (SETFCAP is needed to map uid 0 since kernel 5.12)
	"SETUID", "SETGID", "SETFCAP",
	// image extraction: keep layer file owners, modes, setuid bits and device nodes
	"CHOWN", "DAC_OVERRIDE", "FOWNER", "FSETID", "MKNOD",
	// runc: chroot into and drop caps for inner containers, signal their processes
	"SYS_CHROOT", "SETPCAP", "KILL",
	// Docker's default container caps, so inner containers get the usual set
	"AUDIT_WRITE", "NET_BIND_SERVICE", "NET_RAW",
}

// dindSecurityOpts relax only what rootlesskit needs to create a user namespace and mount procfs.
// no-new-privileges is absent on purpose: it would block newuidmap/newgidmap's file capabilities.
var dindSecurityOpts = []string{
	"apparmor=unconfined",
	"systempaths=unconfined",
}

// dindDevices are the only host devices exposed; slirp4netns needs tun.
var dindDevices = []string{"/dev/net/tun"}

// dindReservedEnvNames point the tool at the sidecar; overriding one would bypass it.
var dindReservedEnvNames = map[string]bool{
	"DOCKER_HOST":       true,
	"DOCKER_TLS_VERIFY": true,
	"DOCKER_CERT_PATH":  true,
	"DOCKER_CONTEXT":    true,
	"DOCKER_CONFIG":     true,
}

// dindHandle identifies the per-run sidecar, its network and its run dir.
type dindHandle struct {
	id          string
	identity    dind.Identity
	container   string
	network     string
	ownsNetwork bool
	runDir      string
}

// newDindHandle derives the per-run resource names without creating anything, so it is safe for dry runs.
// In proxy mode the sidecar joins the proxy's network so its egress is confined too.
func newDindHandle(rs RunSpec) (dindHandle, error) {
	id, err := randID()
	if err != nil {
		return dindHandle{}, err
	}

	identity, err := dind.NewIdentity(hostUserGroup())
	if err != nil {
		return dindHandle{}, err
	}

	h := dindHandle{
		id:        id,
		identity:  identity,
		container: dindHostAlias + "-" + id,
		network:   rs.network,
		runDir:    filepath.Join(rs.ToolHome, dindDirName, id),
	}
	if h.network == "" {
		h.network = dindNetworkPrefix + "-" + id
		h.ownsNetwork = true
	}

	return h, nil
}

// Stop removes the sidecar, its data volume, its own network and the run dir; safe to defer.
func (h dindHandle) Stop() {
	_, _ = dockerRun("rm", "--force", "--volumes", h.container)
	if h.ownsNetwork {
		_, _ = dockerRun("network", "rm", h.network)
	}
	_ = os.RemoveAll(h.runDir)
}

// runArgs builds the `docker run` arguments for the rootless dockerd sidecar.
func (h dindHandle) runArgs(rs RunSpec) []string {
	args := []string{
		// No --rm, so a crashed sidecar keeps its logs for waitReady
		"run", "--detach", "--read-only",
		arg("name", h.container),
		arg("network", h.network),
		arg("network-alias", dindHostAlias),
		label(LabelProject, LabelProjectVal),
		arg("pids-limit", rs.PidsLimit),
		arg("cpus", rs.CPUs),
		arg("memory", rs.Memory),
		// Same user as the tool, so writes to /workspace land as the host user
		arg("user", h.identity.UserGroup()),
		arg("env", "HOME="+dind.Home),
		arg("cap-drop", "ALL"),
	}
	args = append(args, ownerLabels(rs)...)

	for _, capability := range dindCapabilities {
		args = append(args, arg("cap-add", capability))
	}
	args = append(args, arg("security-opt", "seccomp="+filepath.Join(h.runDir, dind.SeccompFileName)))
	for _, opt := range dindSecurityOpts {
		args = append(args, arg("security-opt", opt))
	}
	for _, device := range dindDevices {
		args = append(args, arg("device", device))
	}

	args = append(args,
		// dockerd's runtime dir and scratch space
		arg("tmpfs", "/run:rw,mode=1777"),
		arg("tmpfs", "/tmp:rw,mode=1777"),
		arg("env", "DOCKER_TLS_CERTDIR="+dindCertMountDir),
		arg("volume", h.certVolume(dind.ServerCertSubdir, dindCertMountDir+"/"+dind.ServerCertSubdir)),
		// Client cert for waitReady's probe
		arg("volume", h.certVolume(dind.ClientCertSubdir, dindCertMountDir+"/"+dind.ClientCertSubdir)),
	)

	for _, name := range dind.IdentityFiles {
		args = append(args, arg("volume", mount.NormalizeMountSpec(filepath.Join(h.runDir, dindEtcSubdir, name)+":/etc/"+name)+":ro"))
	}

	for _, volume := range workspaceVolumes(rs) {
		args = append(args, arg("volume", volume))
	}

	if rs.ProxyEnabled {
		args = append(args, proxyEnvArgs(false)...)
	}

	// Trailing flags are passed through to dockerd
	return append(args, rs.DindImage, arg("data-root", dindDataRoot))
}

// toolArgs returns the env and mounts pointing the tool's docker CLI at the sidecar.
func (h dindHandle) toolArgs() []string {
	configDir := mount.NormalizeMountSpec(filepath.Join(h.runDir, dindConfigSubdir) + ":" + dindConfigPath)
	return []string{
		arg("env", "DOCKER_HOST=tcp://"+dindHostAlias+":"+dindPort),
		arg("env", "DOCKER_TLS_VERIFY=1"),
		arg("env", "DOCKER_CERT_PATH="+dindClientCertPath),
		arg("env", "DOCKER_CONFIG="+dindConfigPath),
		arg("volume", h.certVolume(dind.ClientCertSubdir, dindClientCertPath)),
		arg("volume", configDir),
	}
}

// certVolume returns a read-only bind of one per-run cert subdir.
func (h dindHandle) certVolume(subdir, containerPath string) string {
	spec := mount.NormalizeMountSpec(filepath.Join(h.runDir, subdir) + ":" + containerPath)
	return spec + ":ro"
}

// waitReady polls dockerd until it answers, the sidecar exits, the timeout passes or interrupt fires.
func (h dindHandle) waitReady(interrupt <-chan os.Signal) error {
	clientCerts := dindCertMountDir + "/" + dind.ClientCertSubdir
	probe := []string{
		"exec",
		arg("env", "DOCKER_HOST=tcp://localhost:"+dindPort),
		arg("env", "DOCKER_TLS_VERIFY=1"),
		arg("env", "DOCKER_CERT_PATH="+clientCerts),
		h.container,
		"timeout", dindProbeTimeout,
		"docker", "version", arg("format", "{{.Server.Version}}"),
	}

	start := time.Now()
	lastProgress := start
	reason := fmt.Sprintf("did not become ready within %s", dindReadyTimeout)
	for {
		if _, err := dockerRun(probe...); err == nil {
			return nil
		}

		if h.exited() {
			reason = "exited before becoming ready"
			break
		}
		if time.Since(start) > dindReadyTimeout {
			break
		}

		if time.Since(lastProgress) >= dindProgressInterval {
			fmt.Fprintf(os.Stderr, "still waiting for docker sidecar (%s)...\n", time.Since(start).Round(time.Second))
			lastProgress = time.Now()
		}

		select {
		case <-interrupt:
			return fmt.Errorf("docker sidecar startup interrupted")
		case <-time.After(dindPollInterval):
		}
	}

	logs, _ := dockerRun("logs", arg("tail", dindLogTailOnFail), h.container)
	return fmt.Errorf("docker sidecar %s:\n%s", reason, strings.TrimSpace(logs))
}

// exited reports whether the sidecar has stopped or vanished.
func (h dindHandle) exited() bool {
	out, err := dockerRun("inspect", arg("format", "{{.State.Running}}"), h.container)
	return err != nil || strings.TrimSpace(out) == "false"
}

// IsReservedDindEnvName reports whether key is an env var the Docker sidecar manages.
func IsReservedDindEnvName(key string) bool {
	return dindReservedEnvNames[key]
}

// SweepDindResources removes sidecars, networks and run dirs left behind by interrupted runs.
func SweepDindResources(toolHome string) error {
	listContainerArgs := []string{
		"ps", arg("all"), arg("quiet"),
		labelFilter(LabelProject, LabelProjectVal),
		nameFilter(dindHostAlias),
	}
	removeContainerArgs := []string{"rm", arg("force"), arg("volumes")}
	if err := runIfAny(listContainerArgs, removeContainerArgs); err != nil {
		return err
	}

	listNetworkArgs := []string{
		"network", "ls", arg("quiet"),
		labelFilter(LabelProject, LabelProjectVal),
		nameFilter(dindNetworkPrefix),
	}
	removeNetworkArgs := []string{"network", "rm"}
	if err := runIfAny(listNetworkArgs, removeNetworkArgs); err != nil {
		return err
	}

	return sweepDindRunDirs(toolHome)
}

// startDind writes the per-run files, starts the sidecar and waits for dockerd; cleans up on failure.
func startDind(rs RunSpec, interrupt <-chan os.Signal) (dindHandle, error) {
	h, err := newDindHandle(rs)
	if err != nil {
		return dindHandle{}, err
	}

	if err := os.MkdirAll(h.runDir, 0o700); err != nil {
		return dindHandle{}, fmt.Errorf("create dind run dir: %w", err)
	}
	if err := dind.WriteCerts(h.runDir, []string{dindHostAlias, "localhost", "127.0.0.1", "::1"}); err != nil {
		h.Stop()
		return dindHandle{}, err
	}
	if err := writeDindClientConfig(filepath.Join(h.runDir, dindConfigSubdir), rs.ProxyEnabled); err != nil {
		h.Stop()
		return dindHandle{}, err
	}
	if err := dind.WriteSeccompProfile(filepath.Join(h.runDir, dind.SeccompFileName)); err != nil {
		h.Stop()
		return dindHandle{}, err
	}
	if err := dind.WriteIdentityFiles(filepath.Join(h.runDir, dindEtcSubdir), h.identity); err != nil {
		h.Stop()
		return dindHandle{}, err
	}

	if h.ownsNetwork {
		createArgs := []string{"network", "create", label(LabelProject, LabelProjectVal)}
		createArgs = append(createArgs, ownerLabels(rs)...)
		createArgs = append(createArgs, h.network)
		if _, err := dockerRun(createArgs...); err != nil {
			h.Stop()
			return dindHandle{}, fmt.Errorf("create dind network: %w", err)
		}
	}

	if _, err := dockerRun(h.runArgs(rs)...); err != nil {
		h.Stop()
		return dindHandle{}, fmt.Errorf("start docker sidecar: %w", err)
	}

	if err := h.waitReady(interrupt); err != nil {
		h.Stop()
		return dindHandle{}, err
	}

	return h, nil
}

// setupDind starts the Docker sidecar if enabled, returning the tool args and a cleanup func to defer.
func setupDind(rs *RunSpec) (toolArgs []string, cleanup func(), err error) {
	if !rs.DindEnabled {
		return nil, func() {}, nil
	}

	if rs.DryRun {
		return dryRunDind(rs)
	}
	return launchDind(rs)
}

// dryRunDind prints the sidecar command and returns the tool args without provisioning anything.
func dryRunDind(rs *RunSpec) (toolArgs []string, cleanup func(), err error) {
	handle, err := newDindHandle(*rs)
	if err != nil {
		return nil, nil, err
	}
	if handle.ownsNetwork {
		rs.network = handle.network
	}

	if _, err := fmt.Fprintln(os.Stdout, "docker", shellJoin(handle.runArgs(*rs))); err != nil {
		return nil, nil, err
	}
	return handle.toolArgs(), func() {}, nil
}

// launchDind starts the sidecar and returns a cleanup that removes it.
func launchDind(rs *RunSpec) (toolArgs []string, cleanup func(), err error) {
	// Guard first so Ctrl-C during startup still removes the sidecar
	interrupt, stop := guardSignals()

	fmt.Fprintln(os.Stderr, "starting docker sidecar...")
	handle, err := startDind(*rs, interrupt)
	if err != nil {
		stop()
		return nil, nil, err
	}
	rs.network = handle.network

	cleanup = func() {
		handle.Stop()
		stop()
	}
	return handle.toolArgs(), cleanup, nil
}

// sweepDindRunDirs removes run dirs whose sidecar is gone, sparing ones younger than dindRunDirGrace.
func sweepDindRunDirs(toolHome string) error {
	base := filepath.Join(toolHome, dindDirName)
	entries, err := os.ReadDir(base)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read dind dir: %w", err)
	}

	listArgs := []string{
		"ps", arg("all"), arg("format", "{{.Names}}"),
		labelFilter(LabelProject, LabelProjectVal),
		nameFilter(dindHostAlias),
	}
	out, err := dockerRun(listArgs...)
	if err != nil {
		return err
	}

	live := make(map[string]bool)
	for name := range strings.FieldsSeq(out) {
		live[strings.TrimPrefix(name, dindHostAlias+"-")] = true
	}

	for _, entry := range entries {
		if !entry.IsDir() || !dindRunIDPattern.MatchString(entry.Name()) || live[entry.Name()] {
			continue
		}

		info, err := entry.Info()
		if err != nil || time.Since(info.ModTime()) < dindRunDirGrace {
			continue
		}

		if err := os.RemoveAll(filepath.Join(base, entry.Name())); err != nil {
			return fmt.Errorf("remove stale dind run dir: %w", err)
		}
	}

	return nil
}

// workspaceVolumes returns the tool's /workspace volume specs, so the sidecar sees the same paths and :ro overlays.
func workspaceVolumes(rs RunSpec) []string {
	var result []string
	for _, volume := range rs.Volumes {
		expanded := mount.NormalizeMountSpec(mount.ExpandMountSpec(volume, rs.ToolHome, rs.ContainerHome))
		_, rest, ok := mount.SplitHostContainer(expanded)
		if !ok {
			continue
		}

		containerPath, _, _ := strings.Cut(rest, ":")
		if containerPath == mount.WorkspaceContainerPath || strings.HasPrefix(containerPath, mount.WorkspaceContainerPath+"/") {
			result = append(result, expanded)
		}
	}
	return result
}

// writeDindClientConfig creates the tool's DOCKER_CONFIG dir; in proxy mode it sets "proxies" so inner containers and builds use the proxy.
func writeDindClientConfig(dir string, proxyEnabled bool) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create docker config dir: %w", err)
	}
	if !proxyEnabled {
		return nil
	}

	url := "http://" + proxyHostAlias + ":" + proxy.Port
	cfg := map[string]any{
		"proxies": map[string]any{
			"default": map[string]string{
				"httpProxy":  url,
				"httpsProxy": url,
				"noProxy":    "localhost,127.0.0.1",
			},
		},
	}

	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode docker config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), content, 0o600); err != nil {
		return fmt.Errorf("write docker config: %w", err)
	}

	return nil
}
