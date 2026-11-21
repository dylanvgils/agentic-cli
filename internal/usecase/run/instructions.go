package run

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/instructions"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/microsandbox"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// instructionsFacts is what the environment block describes, gathered up front so rendering touches no client.
type instructionsFacts struct {
	caps          sandbox.Capabilities
	image         *docker.ImageInfo
	containerHome string
	mounts        []sandbox.Mount
	ephemeralDirs []sandbox.EphemeralDir
	limits        sandbox.ResourceLimits
	dindLimits    sandbox.ResourceLimits
	proxyMode     sandbox.EgressMode
	dindEnabled   bool
	ports         []sandbox.PortMapping
	rules         []sandbox.AllowRule
	webSearchTool string
	custom        string
}

// prepareInstructions writes content into run id's instructions snapshot and returns its mount spec, or strips any stale
// managed block in place and returns "" when content is empty.
func (s *Service) prepareInstructions(c runConfig, id sandbox.RunID, content string) (string, error) {
	if content == "" {
		return "", c.Tool.Runtime.WriteInstructions(c.Homes.Tool, "")
	}

	hostPath := c.Tool.Runtime.InstructionsHostPath(c.Homes.Tool)
	path := snapshotPath(id, hostPath)
	if err := instructions.PrepareSnapshot(filepath.Join(c.Homes.Tool, tools.ToolsDirName), hostPath, content, path); err != nil {
		return "", err
	}
	return sandbox.VolumeMount(path, c.Tool.Runtime.InstructionsContainerPath), nil
}

// buildInstructions assembles the environment-instructions Markdown for the tool's global instructions file, or "" when disabled via [run.instructions].
func (s *Service) buildInstructions(c runConfig) (string, error) {
	if !instructionsEnabled(c.RC) {
		return "", nil
	}

	facts, err := s.instructionsFacts(c)
	if err != nil {
		return "", err
	}

	return renderInstructions(facts), nil
}

// PreviewInstructions returns the effective content a run of req would write, without prompting or touching any files.
func (s *Service) PreviewInstructions(req Request) (string, error) {
	choice := chooseRuntime(req.Flags, req.Project.RC)
	r, err := s.runner(choice.name, choice.backend)
	if err != nil {
		return "", err
	}
	c, err := s.resolveRun(req, req.Project.RC, choice, r)
	if err != nil {
		return "", err
	}

	content, err := s.buildInstructions(c)
	if err != nil {
		return "", err
	}
	if content == "" {
		return "", nil
	}

	hostPath := c.Tool.Runtime.InstructionsHostPath(c.Homes.Tool)

	return instructions.Merged(filepath.Join(c.Homes.Tool, tools.ToolsDirName), hostPath, content)
}

// instructionsFacts inspects the image and resolves the run's settings into what the block describes.
func (s *Service) instructionsFacts(c runConfig) (instructionsFacts, error) {
	return instructionsFacts{
		caps:          c.Runtime.Caps,
		image:         c.Image,
		containerHome: c.Homes.Container,
		mounts:        c.Mounts.Base,
		ephemeralDirs: c.Tool.Runtime.EphemeralDirs(),
		limits:        c.Limits,
		dindLimits:    c.Dind.Limits,
		proxyMode:     c.Egress.Mode,
		dindEnabled:   c.Dind.Enabled,
		ports:         c.Ports,
		rules:         proxyRules(c),
		webSearchTool: c.Tool.Runtime.WebSearchTool,
		custom:        c.RC.Run.Instructions.Custom,
	}, nil
}

func instructionsEnabled(rc *config.AgenticRC) bool {
	enabled := rc.Run.Instructions.Enabled
	return enabled == nil || *enabled
}

// renderInstructions writes f as the environment-instructions Markdown, one section per writer.
func renderInstructions(f instructionsFacts) string {
	var b strings.Builder
	writeIntro(&b, f.caps.Boundary)
	writePrecedenceSection(&b)
	writeCapabilitiesSection(&b, f.image)
	writeFilesystemSection(&b, f)
	writeResourceLimitsSection(&b, f.limits, f.caps.Features)
	writePrivilegeSection(&b, f.caps.Isolation)
	writeNetworkSection(&b, f)
	writePortsSection(&b, f)
	writeDockerSection(&b, f)
	writeCustomSection(&b, f.custom)

	return b.String()
}

func writeIntro(b *strings.Builder, boundary sandbox.Boundary) {
	if boundary == sandbox.BoundaryVM {
		b.WriteString("# Agentic microVM environment\n\n")
		b.WriteString("You are running inside an agentic-cli microVM with its own kernel. ")
	} else {
		b.WriteString("# Agentic container environment\n\n")
		b.WriteString("You are running inside an agentic-cli container. ")
	}
	b.WriteString("The sections below list what's installed and what's blocked, so you don't try things that fail.\n\n")
}

// writePrecedenceSection scopes the generated block to environment facts, deferring to the project's own instructions file on conflicts.
func writePrecedenceSection(b *strings.Builder) {
	b.WriteString("## Precedence\n\n")
	b.WriteString("agentic-cli writes this block on every run and leaves the rest of this file as is. It describes the environment only. If it conflicts with the project's own instructions file (CLAUDE.md, AGENTS.md, copilot-instructions.md), follow the project's file.\n\n")
}

func writeCapabilitiesSection(b *strings.Builder, info *docker.ImageInfo) {
	b.WriteString("## Installed toolchains\n\n")
	writeSubList(b, "Base toolchain (always installed)", tools.BasePackages())

	if info == nil {
		b.WriteString("(image not built yet - remaining capability details unavailable)\n")
		writeMissingToolsNote(b)
		return
	}

	if info.Base != "" {
		writeSubList(b, "Extra runtimes", strings.Split(info.Base, ","))
	} else {
		b.WriteString("- Extra runtimes: none (base Debian image only)\n")
	}
	if info.Apt != "" {
		writeSubList(b, "Additional apt packages", strings.Split(info.Apt, ","))
	}
	if info.CustomInstalls != "" {
		writeSubList(b, "Custom installs", strings.Split(info.CustomInstalls, ","))
	}
	writeMissingToolsNote(b)
}

// writeSubList writes label as a bullet followed by each item as a nested bullet.
func writeSubList(b *strings.Builder, label string, items []string) {
	fmt.Fprintf(b, "- %s:\n", label)
	for _, item := range items {
		fmt.Fprintf(b, "  - %s\n", item)
	}
}

// writeMissingToolsNote points the agent at telling the user rather than silently trying and failing.
func writeMissingToolsNote(b *strings.Builder) {
	b.WriteString("\nOnly the tools above and the standard Debian utilities (coreutils, `grep`, `sed`, `tar`, ...) are installed. Any other tool isn't, so check with `command -v <tool>` before relying on it. Package managers work only when they install into a writable path (see Filesystem) from a reachable registry, e.g. `npm install` in `/workspace`. For any other missing tool, tell the user which tool you need and why, so they can add it to custom_installs in .agenticrc.toml and rebuild the image.\n\n")
}

// writeFilesystemSection lists each mounted path with its access, then what happens to every other path.
func writeFilesystemSection(b *strings.Builder, f instructionsFacts) {
	b.WriteString("## Filesystem\n\n")
	b.WriteString("- `/workspace`: the project on the user's machine. Writable; changes are real.\n")
	for _, m := range f.mounts {
		writeMountLine(b, m)
	}

	if f.caps.ReadOnlyRoot {
		for _, p := range f.ephemeralDirs {
			fmt.Fprintf(b, "- `%s`: writable, cleared when the container exits.\n", sandbox.ExpandTmpfsSpec(p.Path, f.containerHome))
		}
		b.WriteString("- Every other path is read-only.\n\n")
	} else {
		b.WriteString("- Every other path, `/tmp` included, is writable but discarded when the VM exits.\n\n")
	}
}

// writeMountLine writes a volume's container path and access; the workspace has its own line.
func writeMountLine(b *strings.Builder, m sandbox.Mount) {
	if m.Target == sandbox.WorkspaceContainerPath {
		return
	}

	access := "writable"
	if m.ReadOnly {
		access = "read-only"
	}
	fmt.Fprintf(b, "- `%s`: %s, persists across runs.\n", m.Target, access)
}

// writeResourceLimitsSection leaves out the pids limit for a runtime without one.
func writeResourceLimitsSection(b *strings.Builder, limits sandbox.ResourceLimits, features sandbox.Features) {
	b.WriteString("## Resource limits\n\n")
	if features.PidsLimit {
		fmt.Fprintf(b, "- Max processes: %s\n", limits.PidsLimit)
	}
	fmt.Fprintf(b, "- CPUs: %s\n", limits.CPUs)
	fmt.Fprintf(b, "- Memory: %s\n", limits.Memory)

	b.WriteString("\nThese are hard limits for this run. If a task needs more, tell the user so they can raise ")
	if features.PidsLimit {
		b.WriteString("pids_limit/cpus/memory in .agenticrc.toml (or --pids-limit/--cpus/--memory) and rerun.\n\n")
	} else {
		b.WriteString("cpus/memory in .agenticrc.toml (or --cpus/--memory) and rerun.\n\n")
	}
}

// writePrivilegeSection makes no capability claims for a runtime that keeps the guest's defaults.
func writePrivilegeSection(b *strings.Builder, isolation sandbox.Isolation) {
	b.WriteString("## Privileges\n\n")
	if !isolation.DropsPrivileges {
		b.WriteString("- Non-root user. `sudo` and apt fail.\n\n")
		return
	}

	b.WriteString("- Non-root user, all Linux capabilities dropped, no privilege escalation.\n")
	b.WriteString("- `sudo`, apt and binding ports below 1024 fail.\n\n")
}

// writeNetworkSection is only written when the proxy is enabled and the runtime offers it; in monitor mode nothing is blocked, so it lists no hosts.
func writeNetworkSection(b *strings.Builder, f instructionsFacts) {
	if !f.proxyMode.Enabled() || !f.caps.Proxy {
		return
	}

	b.WriteString("## Network\n\n")
	if f.proxyMode == sandbox.EgressMonitor {
		b.WriteString("- All traffic goes through an egress proxy that logs every request and blocks nothing.\n\n")
		return
	}

	if f.caps.EgressViaProxy {
		b.WriteString("- All traffic goes through an egress proxy.\n")
	} else {
		b.WriteString("- The VM's network policy filters all traffic and inspects HTTPS.\n")
	}

	fmt.Fprintf(b, "- Only these hosts are reachable, and only on ports %s (HTTP and HTTPS):\n", strings.Join(proxy.DefaultPorts, " and "))
	for _, rule := range f.rules {
		if len(rule.Paths) == 0 {
			fmt.Fprintf(b, "  - %s\n", rule.Host)
			continue
		}
		fmt.Fprintf(b, "  - %s, only paths matching %s (`*` is one path segment, `**` the rest)\n", rule.Host, strings.Join(rule.Paths, ", "))
	}

	b.WriteString("- Everything else is blocked: other hosts, other ports and SSH, git over SSH included. ")
	if f.caps.EgressViaProxy {
		b.WriteString("A blocked request gets HTTP 403 from the proxy (curl: `CONNECT tunnel failed, response 403`).\n")
	} else {
		b.WriteString("A blocked request fails with a DNS, connection or TLS error.\n")
	}

	if f.webSearchTool != "" {
		fmt.Fprintf(b, "- Web search (`%s`) runs on the model provider's side and reaches any site. Fetching a page (a fetch tool, `curl`, `wget`) only reaches the hosts above.\n", f.webSearchTool)
	} else {
		b.WriteString("- Web searches and page fetches (any search or fetch tool, `curl`, `wget`) only reach the hosts above.\n")
	}

	b.WriteString("- A blocked host is blocked for every tool. Don't retry it another way (curl, another fetch tool, a mirror).\n")
	b.WriteString("- If a task needs a blocked host, tell the user which host and why, so they can add it to allowed_hosts in .agenticrc.toml and rerun.\n")
	if (sandbox.EgressPolicy{Rules: f.rules}).HasPathRules() {
		b.WriteString("- Other paths on a host limited to some paths get HTTP 403 too; tell the user which path and why, so they can add it to allowed_paths.\n")
	}
	b.WriteString("\n")
}

// writePortsSection is only written when the run forwards ports; it tells the agent how to show a Docker container on one.
func writePortsSection(b *strings.Builder, f instructionsFacts) {
	if len(f.ports) == 0 {
		return
	}

	b.WriteString("## Forwarded ports\n\n")
	b.WriteString("- The user's machine reaches only these ports:\n")
	for _, port := range f.ports {
		fmt.Fprintf(b, "  - `localhost:%d` on the user's machine -> port %d here\n", port.Host, port.Guest)
	}
	b.WriteString("- A server must listen on `0.0.0.0`. On `127.0.0.1` the user gets an empty response.\n")
	b.WriteString("- Once it serves, give the user the matching `localhost` URL from this list.\n")
	b.WriteString("- If a task needs another port, tell the user its number, so they can add it with `--port` and rerun.\n")
	if !f.dindEnabled || !f.caps.Dind {
		b.WriteString("\n")
		return
	}

	example := f.ports[0].Guest
	if f.caps.Boundary == sandbox.BoundaryVM {
		fmt.Fprintf(b, "- To show a Docker container, publish it on a forwarded port. For nginx, which listens on 80: `docker run --detach --publish %d:80 nginx`.\n\n", example)
		return
	}

	fmt.Fprintf(b, "- To show a Docker container, publish it on the sidecar under a forwarded port's number (nginx: `docker run --detach --publish %d:80 nginx`). Then start a relay in the background that listens on `0.0.0.0` on that port here and forwards to `agentic-docker` on the same port. Write it in `python3`, which is always installed.\n", example)
	b.WriteString("\n")
}

// writeDockerSection is only written when the Docker daemon is enabled; a runtime without one only warns there's no daemon.
func writeDockerSection(b *strings.Builder, f instructionsFacts) {
	if !f.dindEnabled {
		return
	}

	b.WriteString("## Docker\n\n")
	if !f.caps.Dind {
		b.WriteString("- There is no Docker daemon in this VM; `docker` commands fail.\n\n")
		return
	}
	if f.caps.Boundary == sandbox.BoundaryVM {
		writeVMDockerSection(b, f)
		return
	}

	b.WriteString("- `docker` uses a rootless daemon in a per-run sidecar. DOCKER_HOST and TLS certs are preset; don't change them.\n")
	b.WriteString("- Bind mounts work only under `/workspace`, which has the same path in the daemon. Other paths, such as `/tmp` or your home, don't exist there.\n")
	fmt.Fprintf(b, "- Published ports are on the sidecar: use `agentic-docker:<port>`, not `localhost`. %s\n", publishedReach(f.ports))
	b.WriteString("- Images, containers and volumes are deleted when this session ends.\n")
	fmt.Fprintf(b, "- Daemon limits, shared by all its containers: %s processes, %s CPUs, %s memory. To raise them, tell the user to set pids_limit/cpus/memory under [run.dind] in .agenticrc.toml (or --dind-pids-limit/--dind-cpus/--dind-memory).\n", f.dindLimits.PidsLimit, f.dindLimits.CPUs, f.dindLimits.Memory)
	writeDaemonAllowlistLine(b, f.proxyMode)
	b.WriteString("\n")
}

// writeVMDockerSection describes the daemon that runs as root inside the microsandbox VM.
func writeVMDockerSection(b *strings.Builder, f instructionsFacts) {
	b.WriteString("- `docker` uses a daemon that runs as root in this VM, with access to any path in it. No setup needed.\n")
	fmt.Fprintf(b, "- Published ports are on `localhost` in the VM. %s\n", publishedReach(f.ports))
	fmt.Fprintf(b, "- Images, containers and volumes live on a %s disk and are deleted when this session ends.\n", microsandbox.DockerDisk)
	b.WriteString("- The daemon shares the VM's CPUs and memory (see Resource limits).\n")
	writeDaemonAllowlistLine(b, f.proxyMode)
	if f.proxyMode.Enabled() && !f.caps.EgressViaProxy {
		b.WriteString("- Containers don't trust the VM's HTTPS inspection CA. For HTTPS inside a container, mount it: `-v /etc/ssl/certs/ca-certificates.crt:/etc/ssl/certs/ca-certificates.crt:ro`. `docker build` steps that use HTTPS fail unless the Dockerfile adds that CA.\n")
	}
	b.WriteString("\n")
}

// publishedReach says whether the user's machine reaches the daemon's published ports, which only a forwarded port can.
func publishedReach(ports []sandbox.PortMapping) string {
	if len(ports) == 0 {
		return "The user's machine can't reach them."
	}
	return "The user's machine reaches them only through a forwarded port (see Forwarded ports)."
}

// writeDaemonAllowlistLine notes that the daemon's traffic is filtered too, when the allowlist applies.
func writeDaemonAllowlistLine(b *strings.Builder, mode sandbox.EgressMode) {
	if mode.Enabled() {
		b.WriteString("- Image pulls, builds and containers use the same allowlist; a registry other than Docker Hub must be added to allowed_hosts.\n")
	}
}

func writeCustomSection(b *strings.Builder, custom string) {
	if custom == "" {
		return
	}

	b.WriteString("## Additional instructions\n\n")
	b.WriteString(custom)
	b.WriteString("\n")
}

// saveSnapshot saves run's instructions snapshot back to its tool's instructions file; without one, or for an unknown
// tool, there is nothing to do.
func saveSnapshot(run sandbox.RunRef) error {
	toolConfig, ok := tools.Configs[run.Tool]
	if !ok {
		return nil
	}

	hostPath := toolConfig.Runtime.InstructionsHostPath(run.ToolHome)
	path := snapshotPath(run.ID, hostPath)
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	if err := instructions.FinalizeSnapshot(filepath.Join(run.ToolHome, tools.ToolsDirName), hostPath, path); err != nil {
		return fmt.Errorf("save instructions: %w", err)
	}
	return nil
}

// snapshotPath returns run id's instructions snapshot in its temp dir, keeping hostPath's extension.
func snapshotPath(id sandbox.RunID, hostPath string) string {
	return filepath.Join(sandbox.RunTempDir(id), "instructions"+filepath.Ext(hostPath))
}
