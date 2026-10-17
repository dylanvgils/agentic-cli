package run

import (
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/mount"
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
	writeCapabilitiesSection(&b, info, caps)
	writeFilesystemSection(&b, toolConfig, containerHome, caps)
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

func writeIntro(b *strings.Builder, runtime Runtime) {
	if runtime == RuntimeMicrosandbox {
		b.WriteString("# Agentic microVM environment\n\n")
		b.WriteString("This tool is running inside an agentic-cli microVM with its own kernel. ")
	} else {
		b.WriteString("# Agentic container environment\n\n")
		b.WriteString("This tool is running inside an agentic-cli container. ")
	}
	b.WriteString("The sections below describe what's installed and what's restricted, so operations that will fail are clear up front.\n\n")
}

// writePrecedenceSection scopes the generated block to environment facts, deferring to the project's own instructions file on conflicts.
func writePrecedenceSection(b *strings.Builder) {
	b.WriteString("## Precedence\n\n")
	b.WriteString("These auto-generated notes describe this environment, not project or personal conventions - they don't apply to anything you or the tool have added to this file yourself. If this project has its own instructions file (CLAUDE.md, AGENTS.md, copilot-instructions.md), follow it whenever it conflicts with anything below.\n\n")
}

func writeCapabilitiesSection(b *strings.Builder, info *docker.ImageInfo, caps sandbox.Capabilities) {
	b.WriteString("## Installed toolchains\n\n")
	writeSubList(b, "Base toolchain (always installed)", tools.BasePackages())

	if info == nil {
		b.WriteString("(image not built yet - remaining capability details unavailable)\n")
		writeMissingToolsNote(b, caps)
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
	writeMissingToolsNote(b, caps)
}

// writeSubList writes label as a bullet followed by each item as a nested bullet.
func writeSubList(b *strings.Builder, label string, items []string) {
	fmt.Fprintf(b, "- %s:\n", label)
	for _, item := range items {
		fmt.Fprintf(b, "  - %s\n", item)
	}
}

// writeMissingToolsNote points the agent at telling the user rather than silently trying and failing.
func writeMissingToolsNote(b *strings.Builder, caps sandbox.Capabilities) {
	if caps.ReadOnlyRoot {
		b.WriteString("\nAnything not listed above cannot be installed or run - sudo, apt, and writes outside /workspace and /tmp are all blocked. ")
	} else {
		b.WriteString("\nAnything not listed above cannot be installed or run - there is no sudo and apt needs root. ")
	}
	b.WriteString("If a task needs a missing tool, tell the user why it's needed so they can add it (e.g. via custom_installs in .agenticrc.toml) and rebuild the image.\n\n")
}

// writeFilesystemSection notes a read-only root with its ephemeral scratch dirs, or a writable root discarded on exit.
func writeFilesystemSection(b *strings.Builder, toolConfig tools.ToolConfig, containerHome string, caps sandbox.Capabilities) {
	b.WriteString("## Filesystem\n\n")
	if caps.ReadOnlyRoot {
		b.WriteString("- The container's root filesystem is read-only.\n")
	}
	b.WriteString("- `/workspace` (the current project) is writable.\n")

	if caps.ReadOnlyRoot {
		for _, p := range toolConfig.Runtime.EphemeralDirs() {
			fmt.Fprintf(b, "- `%s` is writable but ephemeral (cleared when the container exits).\n", mount.ExpandTmpfsSpec(p.Path, containerHome))
		}
	} else {
		b.WriteString("- Everything outside mounted directories, `/tmp` included, is writable but discarded when the VM exits and never reaches the user's machine.\n")
	}

	b.WriteString("- Any mounted secrets under `/run/secrets/` are read-only.\n\n")
}

// writeResourceLimitsSection leaves out the pids limit for a runtime without one.
func writeResourceLimitsSection(b *strings.Builder, limits sandbox.ResourceLimits, caps sandbox.Capabilities) {
	b.WriteString("## Resource limits\n\n")
	if caps.PidsLimit {
		fmt.Fprintf(b, "- Max processes (pids-limit): %s\n", limits.PidsLimit)
	}
	fmt.Fprintf(b, "- CPUs: %s\n", limits.CPUs)
	fmt.Fprintf(b, "- Memory: %s\n", limits.Memory)

	if !caps.PidsLimit {
		b.WriteString("\nThese are configurable, not hard caps - if a task needs more, tell the user why so they can raise cpus/memory in .agenticrc.toml (or the --cpus/--memory flags) and rerun.\n\n")
	} else {
		b.WriteString("\nThese are configurable, not hard caps - if a task needs more, tell the user why so they can raise pids_limit/cpus/memory in .agenticrc.toml (or the --pids-limit/--cpus/--memory flags) and rerun.\n\n")
	}
}

// writePrivilegeSection makes no capability claims for a runtime that keeps the guest's defaults.
func writePrivilegeSection(b *strings.Builder, caps sandbox.Capabilities) {
	b.WriteString("## Privileges\n\n")
	if !caps.DropsPrivileges {
		b.WriteString("- Running as a non-root user; `sudo` and system package installs will not work.\n\n")
		return
	}

	b.WriteString("- Running as a non-root user with all Linux capabilities dropped and no privilege escalation.\n")
	b.WriteString("- `sudo`, system package installs, and binding privileged ports will not work.\n\n")
}

// writeNetworkSection is only written when the proxy is enabled and the runtime offers it; the allowlist itself is omitted in monitor mode, since nothing is actually blocked there.
func writeNetworkSection(b *strings.Builder, toolConfig tools.ToolConfig, rc *config.AgenticRC, in Input, caps sandbox.Capabilities) {
	if !in.ProxyMode.Enabled() || !caps.Proxy {
		return
	}

	b.WriteString("## Network\n\n")
	b.WriteString("- All network egress routes through an egress proxy - there is no direct internet access.\n")

	if in.ProxyMode == sandbox.ProxyMonitor {
		b.WriteString("\n")
		return
	}

	b.WriteString("- Only the following hosts are reachable:\n")
	for _, host := range resolve.ProxyAllowList(toolConfig.Runtime.AllowedHosts, in.DindEnabled, rc) {
		fmt.Fprintf(b, "  - %s\n", host)
	}
	b.WriteString("\nAnything not listed above is blocked. If a task needs a host that isn't reachable, tell the user why so they can add it to allowed_hosts in .agenticrc.toml and rerun.\n\n")
}

// writeDockerSection is only written when the Docker sidecar is enabled; a runtime without one only warns there's no daemon.
func writeDockerSection(b *strings.Builder, in Input, caps sandbox.Capabilities, limits sandbox.ResourceLimits) {
	if !in.DindEnabled {
		return
	}

	b.WriteString("## Docker\n\n")
	if !caps.Dind {
		b.WriteString("- There is no Docker daemon in this VM, so `docker` commands will fail.\n\n")
		return
	}

	b.WriteString("- `docker` talks to a rootless Docker daemon in a separate per-run sidecar; DOCKER_HOST and TLS certs are preconfigured, so don't change them.\n")
	b.WriteString("- `/workspace` is mounted at the same path in the daemon, so bind mounts under `/workspace` work; other paths (e.g. this container's home or `/tmp`) don't exist there.\n")
	b.WriteString("- Published ports listen on the sidecar: reach them at `agentic-docker:<port>`, not `localhost`. They are not reachable from the user's machine.\n")
	b.WriteString("- Images, containers and volumes are discarded when this session ends.\n")
	fmt.Fprintf(b, "- The daemon has its own limits, shared by everything it runs: %s processes, %s CPUs, %s memory. To raise them, tell the user to set pids_limit/cpus/memory under [run.dind] in .agenticrc.toml (or the --dind-pids-limit/--dind-cpus/--dind-memory flags).\n", limits.PidsLimit, limits.CPUs, limits.Memory)
	if in.ProxyMode.Enabled() {
		b.WriteString("- Image pulls, builds and containers go through the same egress proxy and allowlist; a registry other than Docker Hub must be added to allowed_hosts.\n")
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
