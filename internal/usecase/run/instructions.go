package run

import (
	"fmt"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
)

// InstructionsSnapshot is the per-run view of a tool's global instructions file; Cleanup must always be deferred by the caller.
type InstructionsSnapshot struct {
	MountSpec string
	finalize  func() error
}

// Cleanup finalizes the snapshot, warning rather than failing on error.
func (s InstructionsSnapshot) Cleanup() {
	if s.finalize == nil {
		return
	}
	if err := s.finalize(); err != nil {
		logging.Warnf("could not save instructions: %v", err)
	}
}

// PrepareInstructions stages content into a private per-run snapshot file and returns its mount spec, or strips any stale managed block in place when content is empty.
func (s *Service) PrepareInstructions(toolHome string, toolConfig tools.ToolConfig, content string) (InstructionsSnapshot, error) {
	if content == "" {
		return InstructionsSnapshot{}, toolConfig.Runtime.WriteInstructions(toolHome, "")
	}

	hostPath := toolConfig.Runtime.InstructionsHostPath(toolHome)

	snapshotPath, err := tools.PrepareInstructionsSnapshot(toolHome, hostPath, content)
	if err != nil {
		return InstructionsSnapshot{}, err
	}

	return InstructionsSnapshot{
		MountSpec: mount.VolumeMount(snapshotPath, toolConfig.Runtime.InstructionsContainerPath),
		finalize: func() error {
			return tools.FinalizeInstructionsSnapshot(toolHome, hostPath, snapshotPath)
		},
	}, nil
}

// BuildInstructions assembles the environment-instructions Markdown for the tool's global instructions file, or "" when disabled via [run.instructions].
func (s *Service) BuildInstructions(target Target, in Input, toolConfig tools.ToolConfig, rc *config.AgenticRC) (string, error) {
	if !instructionsEnabled(rc) {
		return "", nil
	}

	info, err := s.docker.InspectImage(target.ImageName)
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", target.ImageName, err)
	}

	containerHome := s.docker.ResolveContainerHome(target.ImageName)
	limits := resolve.ResourceLimitsFor(in.Limits, rc)
	dindLimits := resolve.DindResourceLimitsFor(in.DindLimits, rc, limits)

	caps := s.capabilities(in.Runtime)

	var b strings.Builder
	writeIntro(&b, in.Runtime)
	writePrecedenceSection(&b)
	writeCapabilitiesSection(&b, info)
	writeFilesystemSection(&b, toolConfig, in, rc, containerHome, caps)
	writeResourceLimitsSection(&b, limits, caps)
	writePrivilegeSection(&b, caps)
	writeNetworkSection(&b, toolConfig, rc, in, caps)
	writeDockerSection(&b, in, caps, dindLimits)
	writeCustomSection(&b, rc)

	return b.String(), nil
}

// PreviewInstructions returns the effective content a run of req would write, without prompting or touching any files.
func (s *Service) PreviewInstructions(req Request) (string, error) {
	in, err := resolveInput(req.Flags, req.Project.RC, s.capabilities(req.Flags.Runtime))
	if err != nil {
		return "", err
	}

	content, err := s.BuildInstructions(req.Target, in, req.Tool, req.Project.RC)
	if err != nil {
		return "", err
	}
	if content == "" {
		return "", nil
	}

	hostPath := req.Tool.Runtime.InstructionsHostPath(in.ToolHome)

	return tools.MergedInstructions(in.ToolHome, hostPath, content)
}

func instructionsEnabled(rc *config.AgenticRC) bool {
	enabled := rc.Run.Instructions.Enabled
	return enabled == nil || *enabled
}

func writeIntro(b *strings.Builder, runtime sandbox.Runtime) {
	if runtime == sandbox.RuntimeMicrosandbox {
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
func writeFilesystemSection(b *strings.Builder, toolConfig tools.ToolConfig, in Input, rc *config.AgenticRC, containerHome string, caps sandbox.Capabilities) {
	b.WriteString("## Filesystem\n\n")
	b.WriteString("- `/workspace`: the project on the user's machine. Writable; changes are real.\n")
	for _, spec := range resolve.Volumes(toolConfig.Runtime.Mounts(), in.Volumes, rc) {
		writeMountLine(b, spec, containerHome)
	}

	if caps.ReadOnlyRoot {
		for _, p := range toolConfig.Runtime.EphemeralDirs() {
			fmt.Fprintf(b, "- `%s`: writable, cleared when the container exits.\n", mount.ExpandTmpfsSpec(p.Path, containerHome))
		}
		b.WriteString("- Every other path is read-only.\n")
	} else {
		b.WriteString("- Every other path, `/tmp` included, is writable but discarded when the VM exits.\n")
	}

	b.WriteString("- Secrets under `/run/secrets/` are read-only.\n\n")
}

// writeMountLine writes a volume's container path and access; the workspace has its own line.
func writeMountLine(b *strings.Builder, spec, containerHome string) {
	_, target, ok := mount.SplitHostContainer(spec)
	if !ok {
		return
	}

	path, opts, _ := strings.Cut(mount.ExpandTmpfsSpec(target, containerHome), ":")
	if path == mount.WorkspaceContainerPath {
		return
	}

	access := "writable"
	if slices.Contains(strings.Split(opts, ","), "ro") {
		access = "read-only"
	}
	fmt.Fprintf(b, "- `%s`: %s, persists across runs.\n", path, access)
}

// writeResourceLimitsSection leaves out the pids limit for a runtime without one.
func writeResourceLimitsSection(b *strings.Builder, limits sandbox.ResourceLimits, caps sandbox.Capabilities) {
	b.WriteString("## Resource limits\n\n")
	if caps.PidsLimit {
		fmt.Fprintf(b, "- Max processes: %s\n", limits.PidsLimit)
	}
	fmt.Fprintf(b, "- CPUs: %s\n", limits.CPUs)
	fmt.Fprintf(b, "- Memory: %s\n", limits.Memory)

	b.WriteString("\nThese are hard limits for this run. If a task needs more, tell the user so they can raise ")
	if caps.PidsLimit {
		b.WriteString("pids_limit/cpus/memory in .agenticrc.toml (or --pids-limit/--cpus/--memory) and rerun.\n\n")
	} else {
		b.WriteString("cpus/memory in .agenticrc.toml (or --cpus/--memory) and rerun.\n\n")
	}
}

// writePrivilegeSection makes no capability claims for a runtime that keeps the guest's defaults.
func writePrivilegeSection(b *strings.Builder, caps sandbox.Capabilities) {
	b.WriteString("## Privileges\n\n")
	if !caps.DropsPrivileges {
		b.WriteString("- Non-root user. `sudo` and apt fail.\n\n")
		return
	}

	b.WriteString("- Non-root user, all Linux capabilities dropped, no privilege escalation.\n")
	b.WriteString("- `sudo`, apt and binding ports below 1024 fail.\n\n")
}

// writeNetworkSection is only written when the proxy is enabled and the runtime offers it; in monitor mode nothing is blocked, so it lists no hosts.
func writeNetworkSection(b *strings.Builder, toolConfig tools.ToolConfig, rc *config.AgenticRC, in Input, caps sandbox.Capabilities) {
	if !in.ProxyMode.Enabled() || !caps.Proxy {
		return
	}

	b.WriteString("## Network\n\n")
	if in.ProxyMode == sandbox.ProxyMonitor {
		b.WriteString("- All traffic goes through an egress proxy that logs every request and blocks nothing.\n\n")
		return
	}

	if in.Runtime == sandbox.RuntimeMicrosandbox {
		b.WriteString("- The VM's network policy filters all traffic and inspects HTTPS.\n")
	} else {
		b.WriteString("- All traffic goes through an egress proxy.\n")
	}

	fmt.Fprintf(b, "- Only these hosts are reachable, and only on ports %s (HTTP and HTTPS):\n", strings.Join(proxy.DefaultPorts, " and "))
	for _, host := range resolve.ProxyAllowList(toolConfig.Runtime.AllowedHosts, in.DindEnabled, rc) {
		fmt.Fprintf(b, "  - %s\n", host)
	}

	b.WriteString("- Everything else is blocked: other hosts, other ports and SSH, git over SSH included. ")
	if in.Runtime == sandbox.RuntimeMicrosandbox {
		b.WriteString("A blocked request fails with a DNS, connection or TLS error.\n")
	} else {
		b.WriteString("A blocked request gets HTTP 403 from the proxy (curl: `CONNECT tunnel failed, response 403`).\n")
	}

	if search := toolConfig.Runtime.WebSearchTool; search != "" {
		fmt.Fprintf(b, "- Web search (`%s`) runs on the model provider's side and reaches any site. Fetching a page (a fetch tool, `curl`, `wget`) only reaches the hosts above.\n", search)
	} else {
		b.WriteString("- Web searches and page fetches (any search or fetch tool, `curl`, `wget`) only reach the hosts above.\n")
	}

	b.WriteString("- A blocked host is blocked for every tool. Don't retry it another way (curl, another fetch tool, a mirror).\n")
	b.WriteString("- If a task needs a blocked host, tell the user which host and why, so they can add it to allowed_hosts in .agenticrc.toml and rerun.\n\n")
}

// writeDockerSection is only written when the Docker sidecar is enabled; a runtime without one only warns there's no daemon.
func writeDockerSection(b *strings.Builder, in Input, caps sandbox.Capabilities, limits sandbox.ResourceLimits) {
	if !in.DindEnabled {
		return
	}

	b.WriteString("## Docker\n\n")
	if !caps.Dind {
		b.WriteString("- There is no Docker daemon in this VM; `docker` commands fail.\n\n")
		return
	}

	b.WriteString("- `docker` uses a rootless daemon in a per-run sidecar. DOCKER_HOST and TLS certs are preset; don't change them.\n")
	b.WriteString("- Bind mounts work only under `/workspace`, which has the same path in the daemon. Other paths, such as `/tmp` or your home, don't exist there.\n")
	b.WriteString("- Published ports are on the sidecar: use `agentic-docker:<port>`, not `localhost`. The user's machine can't reach them.\n")
	b.WriteString("- Images, containers and volumes are deleted when this session ends.\n")
	fmt.Fprintf(b, "- Daemon limits, shared by all its containers: %s processes, %s CPUs, %s memory. To raise them, tell the user to set pids_limit/cpus/memory under [run.dind] in .agenticrc.toml (or --dind-pids-limit/--dind-cpus/--dind-memory).\n", limits.PidsLimit, limits.CPUs, limits.Memory)
	if in.ProxyMode.Enabled() {
		b.WriteString("- Image pulls, builds and containers use the same allowlist; a registry other than Docker Hub must be added to allowed_hosts.\n")
	}
	b.WriteString("\n")
}

func writeCustomSection(b *strings.Builder, rc *config.AgenticRC) {
	custom := rc.Run.Instructions.Custom
	if custom == "" {
		return
	}

	b.WriteString("## Additional instructions\n\n")
	b.WriteString(custom)
	b.WriteString("\n")
}
