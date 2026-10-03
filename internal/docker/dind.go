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
	// dindHostAlias is the sidecar's stable network alias, so DOCKER_HOST is the same on every run.
	dindHostAlias = "agentic-docker"

	// dindNetworkPrefix names the per-run network created when no proxy network exists to join.
	dindNetworkPrefix = "agentic-dind"

	// dindPort is dockerd's TLS port; the image's entrypoint only listens there when TLS certs are present.
	dindPort = "2376"

	// dindDirName is the subdirectory under the agentic home holding per-run dirs (certs, docker config, seccomp profile).
	dindDirName = "dind"

	// dindCertMountDir is the sidecar's DOCKER_TLS_CERTDIR.
	dindCertMountDir = "/certs"

	// dindClientCertPath is where the tool container sees its client cert.
	dindClientCertPath = "/run/secrets/docker"

	// dindEtcSubdir holds the generated /etc identity files (see dind.Identity).
	dindEtcSubdir = "etc"

	// dindConfigSubdir holds the tool's writable DOCKER_CONFIG (buildx state, logins), mounted at dindConfigPath.
	dindConfigSubdir = "config"
	dindConfigPath   = "/run/docker-config"

	// dindDataRoot is a subdir of the image's VOLUME (mode 1777, owned by the image user) that dockerd creates
	// itself, so it owns it and can chmod it; the VOLUME root is unmapped in the user namespace unless the host uid is 1000.
	dindDataRoot = dind.Home + "/.local/share/docker/data"

	// dindProbeTimeout bounds each in-sidecar readiness probe (seconds), so a dockerd that accepts but never answers can't stall it.
	dindProbeTimeout = "10"
)

// hostUserGroup is a test-stubbable indirection into platform.UserGroup.
var hostUserGroup = platform.UserGroup

var (
	// Readiness wait for dockerd, progress cadence, and how much sidecar log to show if it never answers; vars so tests can shorten them.
	dindReadyTimeout     = 90 * time.Second
	dindPollInterval     = time.Second
	dindProgressInterval = 15 * time.Second
	dindLogTailOnFail    = "30"
)

// dindRunDirGrace spares a run dir whose sidecar isn't started yet from a concurrent run's sweep; a var so tests can shorten it.
var dindRunDirGrace = 10 * time.Minute

// dindRunIDPattern matches randID output, so the run-dir sweep never touches anything agentic didn't create.
var dindRunIDPattern = regexp.MustCompile(`^[0-9a-f]{12}$`)

// dindCapabilities is the sidecar's capability bounding set. Rootless dockerd starts as the host user
// with no effective capabilities on the host; these only become usable inside the user namespace
// rootlesskit creates, where dockerd and runc need them to set up inner containers. SETUID/SETGID
// also back the newuidmap/newgidmap file capabilities that write the namespace's id maps.
var dindCapabilities = []string{
	"AUDIT_WRITE", "CHOWN", "DAC_OVERRIDE", "FOWNER", "FSETID", "KILL", "MKNOD",
	"NET_ADMIN", "NET_BIND_SERVICE", "NET_RAW", "SETFCAP", "SETGID", "SETPCAP",
	"SETUID", "SYS_ADMIN", "SYS_CHROOT",
}

// dindSecurityOpts relax only what rootlesskit needs to create a user namespace and mount procfs
// inside it (Docker's AppArmor profile denies mount); seccomp stays on via a derived profile
// (see dind.WriteSeccompProfile). no-new-privileges is deliberately absent: it would stop newuidmap
// and newgidmap from gaining their file capabilities.
var dindSecurityOpts = []string{
	"apparmor=unconfined",
	"systempaths=unconfined",
}

// dindDevices are the only host devices exposed; slirp4netns needs a tun device for inner-container networking.
var dindDevices = []string{"/dev/net/tun"}

// dindReservedEnvNames are env vars pointing the tool at the sidecar; overriding one would silently bypass it.
var dindReservedEnvNames = map[string]bool{
	"DOCKER_HOST":       true,
	"DOCKER_TLS_VERIFY": true,
	"DOCKER_CERT_PATH":  true,
	"DOCKER_CONTEXT":    true,
	"DOCKER_CONFIG":     true,
}

// dindHandle identifies the per-run Docker sidecar, the network it shares with the tool, and its host-side run dir.
type dindHandle struct {
	id          string
	identity    dind.Identity
	container   string
	network     string
	ownsNetwork bool
	runDir      string
}

// newDindHandle derives the per-run sidecar resource names without creating anything, so it is safe for dry runs.
// In proxy mode the sidecar joins the proxy's internal network, so its egress is confined the same way.
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

// Stop removes the sidecar (and its anonymous data volume), its network if this run created it, and the run dir; idempotent and error-ignoring, so it is safe to defer.
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
		// No --rm: a crashed sidecar keeps its logs for waitReady; Stop and SweepDindResources remove it
		"run", "--detach", "--read-only",
		arg("name", h.container),
		arg("network", h.network),
		arg("network-alias", dindHostAlias),
		label(LabelProject, LabelProjectVal),
		arg("pids-limit", rs.PidsLimit),
		arg("cpus", rs.CPUs),
		arg("memory", rs.Memory),
		// Same identity as the tool container: inner-container writes to /workspace land as the host user
		arg("user", h.identity.UserGroup()),
		arg("env", "HOME="+dind.Home),
		arg("cap-drop", "ALL"),
	}

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
		// dockerd's runtime dir (XDG_RUNTIME_DIR=/run/user/<uid>) and scratch space; the data-root lives in the image's VOLUME
		arg("tmpfs", "/run:rw,mode=1777"),
		arg("tmpfs", "/tmp:rw,mode=1777"),
		arg("env", "DOCKER_TLS_CERTDIR="+dindCertMountDir),
		arg("volume", h.certVolume(dind.ServerCertSubdir, dindCertMountDir+"/"+dind.ServerCertSubdir)),
		// The client cert lets waitReady probe dockerd over the same TLS path the tool uses
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

	// Trailing flags are passed through to dockerd by the image's entrypoint
	return append(args, rs.DindImage, arg("data-root", dindDataRoot))
}

// toolArgs returns the env, read-only client-cert mount and writable config mount pointing the tool's docker CLI at the sidecar.
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

// waitReady polls dockerd over TLS until it answers, failing early if the sidecar exits, dindReadyTimeout
// passes, or interrupt fires; attaches the sidecar's log tail on failure.
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

// exited reports whether the sidecar has stopped or vanished, so waitReady stops polling a dead daemon.
func (h dindHandle) exited() bool {
	out, err := dockerRun("inspect", arg("format", "{{.State.Running}}"), h.container)
	return err != nil || strings.TrimSpace(out) == "false"
}

// IsReservedDindEnvName reports whether key is an env var the Docker sidecar manages.
func IsReservedDindEnvName(key string) bool {
	return dindReservedEnvNames[key]
}

// SweepDindResources idempotently removes leftover sidecars (with their data volumes), per-run
// networks and per-run dirs under toolHome from interrupted runs, scoped to agentic-managed resources.
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

// startDind generates per-run certs, provisions the network (unless joining the proxy's), starts
// the sidecar and waits for dockerd (abandoned if interrupt fires); cleans up whatever it created on any failure.
func startDind(rs RunSpec, interrupt <-chan os.Signal) (dindHandle, error) {
	// A crashed run never reaches Stop, so heal its leftovers (keys, registry logins) here instead of waiting for `agentic clean`
	if err := sweepDindRunDirs(rs.ToolHome); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not remove stale docker sidecar dirs: %v\n", err)
	}

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
		createArgs := []string{"network", "create", label(LabelProject, LabelProjectVal), h.network}
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

// sweepDindRunDirs removes per-run dirs under toolHome whose sidecar container is gone, sparing
// ones younger than dindRunDirGrace; a missing dind dir is a no-op.
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

// workspaceVolumes returns the tool's volume specs targeting /workspace (the project bind plus any
// read-only overlays), in order, so the sidecar sees the same paths with the same write protection.
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

// writeDindClientConfig creates the tool's DOCKER_CONFIG dir. In proxy mode its config.json sets
// the CLI's "proxies" so containers and builds started through the sidecar get the proxy env.
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
