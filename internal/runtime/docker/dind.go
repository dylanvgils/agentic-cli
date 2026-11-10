package docker

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/dind"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

const (
	// dindHostAlias is the sidecar's network alias, so DOCKER_HOST is stable across runs.
	dindHostAlias = "agentic-docker"

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

	// dryRunAddressPoolPlaceholder stands in for the address pool in dry-run output, as the run network isn't created.
	dryRunAddressPoolPlaceholder = "<pool outside the run network>"
)

// dindLogTailOnFail is how many sidecar log lines a failed readiness wait shows.
const dindLogTailOnFail = "30"

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

// dindHandle identifies the per-run sidecar and its run dir.
type dindHandle struct {
	client    *Client
	id        sandbox.RunID
	identity  dind.Identity
	container string
	runDir    string
	// addressPool is dockerd's --default-address-pool
	addressPool string
}

// newDindHandle derives the per-run resource names without creating anything, so it is safe for dry runs.
func (c *Client) newDindHandle(rs toolRun) (dindHandle, error) {
	if rs.User == nil {
		return dindHandle{}, errNoHostUser
	}
	identity, err := dind.NewIdentity(*rs.User)
	if err != nil {
		return dindHandle{}, err
	}

	return dindHandle{
		client:      c,
		id:          rs.RunID,
		identity:    identity,
		container:   dindContainerName(rs.RunID),
		runDir:      dindRunDir(rs.ToolHome, rs.RunID),
		addressPool: dryRunAddressPoolPlaceholder,
	}, nil
}

// runArgs builds the `docker run` arguments for the rootless dockerd sidecar, on rs's run network so with the proxy
// its egress is confined too.
func (h dindHandle) runArgs(rs toolRun) []string {
	args := []string{
		// No --rm, so a crashed sidecar keeps its logs for waitReady
		"run", "--detach", "--read-only",
		arg("name", h.container),
		arg("network", rs.network),
		arg("network-alias", dindHostAlias),
		label(sandbox.LabelProject, sandbox.LabelProjectVal),
		arg("pids-limit", rs.Dind.Limits.PidsLimit),
		arg("cpus", rs.Dind.Limits.CPUs),
		arg("memory", rs.Dind.Limits.Memory),
		// Same user as the tool, so writes to /workspace land as the host user
		arg("user", h.identity.UserGroup()),
		arg("env", "HOME="+dind.Home),
		arg("cap-drop", "ALL"),
	}
	args = append(args, ownerLabels(rs.container, rs.RunID)...)

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
		args = append(args, arg("volume", sandbox.NormalizeMountSpec(filepath.Join(h.runDir, dindEtcSubdir, name)+":/etc/"+name)+":ro"))
	}

	for _, volume := range workspaceVolumes(rs.RunSpec) {
		args = append(args, arg("volume", volume))
	}

	if rs.Egress.Mode.Enabled() {
		args = append(args, proxyEnvArgs(false)...)
	}

	// Trailing flags are passed through to dockerd
	return append(args, tools.DindImage, arg("data-root", dindDataRoot), arg("default-address-pool", h.addressPool))
}

// toolArgs returns the env and mounts pointing the tool's docker CLI at the sidecar.
func (h dindHandle) toolArgs() []string {
	configDir := sandbox.NormalizeMountSpec(filepath.Join(h.runDir, dindConfigSubdir) + ":" + dindConfigPath)
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
	spec := sandbox.NormalizeMountSpec(filepath.Join(h.runDir, subdir) + ":" + containerPath)
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

	timing := h.client.timing
	start := time.Now()
	lastProgress := start
	reason := fmt.Sprintf("did not become ready within %s", timing.dindReadyTimeout)
	for {
		if _, err := h.client.run(probe...); err == nil {
			return nil
		}

		if h.exited() {
			reason = "exited before becoming ready"
			break
		}
		if time.Since(start) > timing.dindReadyTimeout {
			break
		}

		if time.Since(lastProgress) >= timing.dindProgressInterval {
			logging.Infof("still waiting for docker sidecar (%s)...", time.Since(start).Round(time.Second))
			lastProgress = time.Now()
		}

		select {
		case <-interrupt:
			return fmt.Errorf("docker sidecar startup interrupted")
		case <-time.After(timing.dindPollInterval):
		}
	}

	logs, _ := h.client.run("logs", arg("tail", dindLogTailOnFail), h.container)
	return fmt.Errorf("docker sidecar %s:\n%s", reason, strings.TrimSpace(logs))
}

// exited reports whether the sidecar has stopped or vanished.
func (h dindHandle) exited() bool {
	out, err := h.client.run("inspect", arg("format", "{{.State.Running}}"), h.container)
	return err != nil || strings.TrimSpace(out) == "false"
}

// startDind writes the run files, starts the sidecar and waits for dockerd.
func (c *Client) startDind(rs toolRun, interrupt <-chan os.Signal) (dindHandle, error) {
	h, err := c.newDindHandle(rs)
	if err != nil {
		return dindHandle{}, err
	}

	if h.addressPool, err = c.dindAddressPool(rs.network); err != nil {
		return dindHandle{}, err
	}

	if err := os.MkdirAll(h.runDir, 0o700); err != nil {
		return dindHandle{}, fmt.Errorf("create dind run dir: %w", err)
	}
	if err := dind.WriteCerts(h.runDir, []string{dindHostAlias, "localhost", "127.0.0.1", "::1"}); err != nil {
		return dindHandle{}, err
	}
	if err := writeDindClientConfig(filepath.Join(h.runDir, dindConfigSubdir), rs.Egress.Mode.Enabled()); err != nil {
		return dindHandle{}, err
	}
	if err := dind.WriteSeccompProfile(filepath.Join(h.runDir, dind.SeccompFileName)); err != nil {
		return dindHandle{}, err
	}
	if err := dind.WriteIdentityFiles(filepath.Join(h.runDir, dindEtcSubdir), h.identity); err != nil {
		return dindHandle{}, err
	}

	if _, err := c.run(h.runArgs(rs)...); err != nil {
		return dindHandle{}, fmt.Errorf("start docker sidecar: %w", err)
	}

	if err := h.waitReady(interrupt); err != nil {
		return dindHandle{}, err
	}

	return h, nil
}

// dindAddressPool returns an address pool for inner networks that overlaps none of network's subnets.
func (c *Client) dindAddressPool(network string) (string, error) {
	subnets, err := c.networkSubnets(network)
	if err != nil {
		return "", err
	}
	return dind.AddressPool(subnets)
}

// setupDind starts the Docker sidecar if enabled and returns the tool args.
func (c *Client) setupDind(rs toolRun, interrupt <-chan os.Signal) (toolArgs []string, err error) {
	if !rs.Dind.Enabled {
		return nil, nil
	}

	if rs.DryRun {
		return c.dryRunDind(rs)
	}
	return c.launchDind(rs, interrupt)
}

// dryRunDind prints the sidecar command and returns the tool args without provisioning anything.
func (c *Client) dryRunDind(rs toolRun) (toolArgs []string, err error) {
	handle, err := c.newDindHandle(rs)
	if err != nil {
		return nil, err
	}

	sandbox.PrintDryRun("docker sidecar", "docker", handle.runArgs(rs))
	return handle.toolArgs(), nil
}

// launchDind starts the sidecar and returns the tool args.
func (c *Client) launchDind(rs toolRun, interrupt <-chan os.Signal) (toolArgs []string, err error) {
	logging.Info("starting docker sidecar...")
	handle, err := c.startDind(rs, interrupt)
	if err != nil {
		return nil, err
	}
	return handle.toolArgs(), nil
}

// sweepDindRunDirs tears down runs left with only a run dir older than dindRunDirGrace.
func (c *Client) sweepDindRunDirs(toolHome string, resources []ownedResource) error {
	base := filepath.Join(toolHome, dindDirName)
	entries, err := os.ReadDir(base)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read dind dir: %w", err)
	}

	live := liveRunIDs(resources)
	var errs []error
	for _, entry := range entries {
		if id, ok := staleRunDir(entry, live, c.timing.dindRunDirGrace); ok {
			errs = append(errs, c.Teardown(toolHome, id))
		}
	}
	return errors.Join(errs...)
}

// workspaceVolumes returns the tool's /workspace volume specs, so the sidecar sees the same paths and :ro overlays.
func workspaceVolumes(rs sandbox.RunSpec) []string {
	var result []string
	for _, volume := range rs.Volumes {
		expanded := sandbox.NormalizeMountSpec(sandbox.ExpandMountSpec(volume, rs.ToolHome, rs.ContainerHome))
		_, rest, ok := sandbox.SplitHostContainer(expanded)
		if !ok {
			continue
		}

		containerPath, _, _ := strings.Cut(rest, ":")
		if containerPath == sandbox.WorkspaceContainerPath || strings.HasPrefix(containerPath, sandbox.WorkspaceContainerPath+"/") {
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

	cfg := map[string]any{
		"proxies": map[string]any{
			"default": map[string]string{
				"httpProxy":  proxyURL,
				"httpsProxy": proxyURL,
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

// dindContainerName returns run id's dind sidecar name.
func dindContainerName(id sandbox.RunID) string {
	return dindHostAlias + "-" + id.String()
}

// dindRunDir returns run id's host dir under toolHome.
func dindRunDir(toolHome string, id sandbox.RunID) string {
	return filepath.Join(toolHome, dindDirName, id.String())
}

// liveRunIDs returns the runs resources belong to, by label or an older version's dind sidecar name.
func liveRunIDs(resources []ownedResource) map[sandbox.RunID]bool {
	live := make(map[sandbox.RunID]bool, len(resources))
	for _, r := range resources {
		live[r.run] = true
		if id, ok := strings.CutPrefix(r.name, dindHostAlias+"-"); ok {
			live[sandbox.RunID(id)] = true
		}
	}
	return live
}

// staleRunDir returns entry's run when it's a run dir of no live run, older than grace.
func staleRunDir(entry fs.DirEntry, live map[sandbox.RunID]bool, grace time.Duration) (sandbox.RunID, bool) {
	id, err := sandbox.ParseRunID(entry.Name())
	if err != nil || !entry.IsDir() || live[id] {
		return "", false
	}

	info, err := entry.Info()
	if err != nil || time.Since(info.ModTime()) < grace {
		return "", false
	}
	return id, true
}
