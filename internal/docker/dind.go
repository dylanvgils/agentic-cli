package docker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/mount"
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

	// dindEtcSubdir holds the generated /etc identity files (see dindIdentity).
	dindEtcSubdir = "etc"

	// dindConfigSubdir holds the tool's writable DOCKER_CONFIG (buildx state, logins), mounted at dindConfigPath.
	dindConfigSubdir = "config"
	dindConfigPath   = "/run/docker-config"
)

var (
	// Readiness wait for dockerd, and how much sidecar log to show if it never answers; vars so tests can shorten them.
	dindReadyTimeout  = 90 * time.Second
	dindPollInterval  = time.Second
	dindLogTailOnFail = "30"
)

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
// (see dindSeccompProfile). no-new-privileges is deliberately absent: it would stop newuidmap
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
	identity    dindIdentity
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

	identity, err := currentDindIdentity()
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
		"run", "--detach", "--rm", "--read-only",
		arg("name", h.container),
		arg("network", h.network),
		arg("network-alias", dindHostAlias),
		label(LabelProject, LabelProjectVal),
		arg("pids-limit", rs.PidsLimit),
		arg("cpus", rs.CPUs),
		arg("memory", rs.Memory),
		// Same identity as the tool container: inner-container writes to /workspace land as the host user
		arg("user", h.identity.userGroup()),
		arg("env", "HOME="+dindHome),
		arg("cap-drop", "ALL"),
	}

	for _, capability := range dindCapabilities {
		args = append(args, arg("cap-add", capability))
	}
	args = append(args, arg("security-opt", "seccomp="+filepath.Join(h.runDir, dindSeccompFileName)))
	for _, opt := range dindSecurityOpts {
		args = append(args, arg("security-opt", opt))
	}
	for _, device := range dindDevices {
		args = append(args, arg("device", device))
	}

	args = append(args,
		// dockerd's runtime dir (XDG_RUNTIME_DIR=/run/user/<uid>) and scratch space; the data-root is the image's VOLUME
		arg("tmpfs", "/run:rw,mode=1777"),
		arg("tmpfs", "/tmp:rw,mode=1777"),
		arg("env", "DOCKER_TLS_CERTDIR="+dindCertMountDir),
		arg("volume", h.certVolume(dindServerCertSubdir, dindCertMountDir+"/"+dindServerCertSubdir)),
		// The client cert lets waitReady probe dockerd over the same TLS path the tool uses
		arg("volume", h.certVolume(dindClientCertSubdir, dindCertMountDir+"/"+dindClientCertSubdir)),
	)

	for _, name := range dindIdentityFiles {
		args = append(args, arg("volume", mount.NormalizeMountSpec(filepath.Join(h.runDir, dindEtcSubdir, name)+":/etc/"+name)+":ro"))
	}

	for _, volume := range workspaceVolumes(rs) {
		args = append(args, arg("volume", volume))
	}

	if rs.ProxyEnabled {
		args = append(args, proxyEnvArgs(false)...)
	}

	return append(args, rs.DindImage)
}

// toolArgs returns the env, read-only client-cert mount and writable config mount pointing the tool's docker CLI at the sidecar.
func (h dindHandle) toolArgs() []string {
	configDir := mount.NormalizeMountSpec(filepath.Join(h.runDir, dindConfigSubdir) + ":" + dindConfigPath)
	return []string{
		arg("env", "DOCKER_HOST=tcp://"+dindHostAlias+":"+dindPort),
		arg("env", "DOCKER_TLS_VERIFY=1"),
		arg("env", "DOCKER_CERT_PATH="+dindClientCertPath),
		arg("env", "DOCKER_CONFIG="+dindConfigPath),
		arg("volume", h.certVolume(dindClientCertSubdir, dindClientCertPath)),
		arg("volume", configDir),
	}
}

// certVolume returns a read-only bind of one per-run cert subdir.
func (h dindHandle) certVolume(subdir, containerPath string) string {
	spec := mount.NormalizeMountSpec(filepath.Join(h.runDir, subdir) + ":" + containerPath)
	return spec + ":ro"
}

// waitReady polls dockerd over TLS until it answers or dindReadyTimeout passes, attaching the sidecar's log tail on failure.
func (h dindHandle) waitReady() error {
	clientCerts := dindCertMountDir + "/" + dindClientCertSubdir
	probe := []string{
		"exec",
		arg("env", "DOCKER_HOST=tcp://localhost:"+dindPort),
		arg("env", "DOCKER_TLS_VERIFY=1"),
		arg("env", "DOCKER_CERT_PATH="+clientCerts),
		h.container,
		"docker", "version", arg("format", "{{.Server.Version}}"),
	}

	deadline := time.Now().Add(dindReadyTimeout)
	for {
		if _, err := dockerRun(probe...); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(dindPollInterval)
	}

	logs, _ := dockerRun("logs", arg("tail", dindLogTailOnFail), h.container)
	return fmt.Errorf("docker sidecar did not become ready within %s:\n%s", dindReadyTimeout, strings.TrimSpace(logs))
}

// IsReservedDindEnvName reports whether key is an env var the Docker sidecar manages.
func IsReservedDindEnvName(key string) bool {
	return dindReservedEnvNames[key]
}

// SweepDindResources idempotently removes leftover sidecars (with their data volumes) and per-run
// networks from interrupted runs, scoped to agentic-managed resources.
func SweepDindResources() error {
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
	return runIfAny(listNetworkArgs, removeNetworkArgs)
}

// startDind generates per-run certs, provisions the network (unless joining the proxy's), starts
// the sidecar and waits for dockerd; cleans up whatever it created on any failure.
func startDind(rs RunSpec) (dindHandle, error) {
	h, err := newDindHandle(rs)
	if err != nil {
		return dindHandle{}, err
	}

	if err := os.MkdirAll(h.runDir, 0o700); err != nil {
		return dindHandle{}, fmt.Errorf("create dind run dir: %w", err)
	}
	if err := writeDindCerts(h.runDir, []string{dindHostAlias, "localhost", "127.0.0.1", "::1"}); err != nil {
		h.Stop()
		return dindHandle{}, err
	}
	if err := writeDindClientConfig(filepath.Join(h.runDir, dindConfigSubdir), rs.ProxyEnabled); err != nil {
		h.Stop()
		return dindHandle{}, err
	}
	if err := writeDindSeccompProfile(filepath.Join(h.runDir, dindSeccompFileName)); err != nil {
		h.Stop()
		return dindHandle{}, err
	}
	if err := writeDindIdentityFiles(filepath.Join(h.runDir, dindEtcSubdir), h.identity); err != nil {
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

	if err := h.waitReady(); err != nil {
		h.Stop()
		return dindHandle{}, err
	}

	return h, nil
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
