package run

import (
	"fmt"
	"os"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/tools"
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
		fmt.Fprintf(os.Stderr, "warning: could not save instructions: %v\n", err)
	}
}

// PrepareInstructions stages content into a private per-run snapshot file and returns its mount spec, or strips any stale managed block in place when content is empty.
func PrepareInstructions(toolHome string, toolConfig tools.ToolConfig, content string) (InstructionsSnapshot, error) {
	if content == "" {
		return InstructionsSnapshot{}, toolConfig.Runtime.WriteInstructions(toolHome, "")
	}

	hostPath := toolConfig.Runtime.InstructionsHostPath(toolHome)

	snapshotPath, err := tools.PrepareInstructionsSnapshot(hostPath, content)
	if err != nil {
		return InstructionsSnapshot{}, err
	}

	return InstructionsSnapshot{
		MountSpec: mount.VolumeMount(snapshotPath, toolConfig.Runtime.InstructionsContainerPath),
		finalize: func() error {
			return tools.FinalizeInstructionsSnapshot(hostPath, snapshotPath)
		},
	}, nil
}

// BuildInstructions assembles the environment-instructions Markdown for the tool's global instructions file, or "" when disabled via [run.instructions].
func BuildInstructions(target Target, in Input, toolConfig tools.ToolConfig, rc *config.AgenticRC) (string, error) {
	if !instructionsEnabled(rc) {
		return "", nil
	}

	info, err := InspectImage(target.ImageName)
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", target.ImageName, err)
	}

	containerHome := docker.ResolveContainerHome(target.ImageName)
	limits := resolveResourceLimits(in.PidsLimit, in.CPUs, in.Memory, rc)

	var b strings.Builder
	b.WriteString("# Agentic container environment\n\n")
	b.WriteString("This tool is running inside an agentic-cli container. The sections below describe what's installed and what's restricted, so operations that will fail are clear up front.\n\n")

	writePrecedenceSection(&b)
	writeCapabilitiesSection(&b, info)
	writeFilesystemSection(&b, toolConfig, containerHome)
	writeResourceLimitsSection(&b, limits)
	writePrivilegeSection(&b)
	writeNetworkSection(&b, toolConfig, rc, in)
	writeCustomSection(&b, rc)

	return b.String(), nil
}

// PreviewInstructions returns the effective content a run would write, without touching any files.
func PreviewInstructions(target Target, in Input, toolConfig tools.ToolConfig, rc *config.AgenticRC) (string, error) {
	content, err := BuildInstructions(target, in, toolConfig, rc)
	if err != nil {
		return "", err
	}
	if content == "" {
		return "", nil
	}

	hostPath := toolConfig.Runtime.InstructionsHostPath(in.ToolHome)

	return tools.MergedInstructions(hostPath, content)
}

func instructionsEnabled(rc *config.AgenticRC) bool {
	enabled := rc.Run.Instructions.Enabled
	return enabled == nil || *enabled
}

// writePrecedenceSection scopes the generated block to environment facts, deferring to the project's own instructions file on conflicts.
func writePrecedenceSection(b *strings.Builder) {
	b.WriteString("## Precedence\n\n")
	b.WriteString("These auto-generated notes describe the container environment, not project or personal conventions - they don't apply to anything you or the tool have added to this file yourself. If this project has its own instructions file (CLAUDE.md, AGENTS.md, copilot-instructions.md), follow it whenever it conflicts with anything below.\n\n")
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
	b.WriteString("\nAnything not listed above cannot be installed or run - sudo, apt, and writes outside /workspace and /tmp are all blocked. If a task needs a missing tool, tell the user why it's needed so they can add it (e.g. via custom_installs in .agenticrc.toml) and rebuild the image.\n\n")
}

func writeFilesystemSection(b *strings.Builder, toolConfig tools.ToolConfig, containerHome string) {
	b.WriteString("## Filesystem\n\n")
	b.WriteString("- The container's root filesystem is read-only.\n")
	b.WriteString("- `/workspace` (the current project) is writable.\n")

	for _, spec := range toolConfig.Runtime.TmpfsMounts() {
		fmt.Fprintf(b, "- `%s` is writable but ephemeral (cleared when the container exits).\n", tmpfsPath(spec, containerHome))
	}

	b.WriteString("- Any mounted secrets under `/run/secrets/` are read-only.\n\n")
}

// tmpfsPath extracts the container-side path from a tmpfs mount spec, expanding $CONTAINER_HOME.
func tmpfsPath(spec, containerHome string) string {
	expanded := mount.ExpandTmpfsSpec(spec, containerHome)
	path, _, _ := strings.Cut(expanded, ":")
	return path
}

func writeResourceLimitsSection(b *strings.Builder, limits resourceLimits) {
	b.WriteString("## Resource limits\n\n")
	fmt.Fprintf(b, "- Max processes (pids-limit): %s\n", limits.pidsLimit)
	fmt.Fprintf(b, "- CPUs: %s\n", limits.cpus)
	fmt.Fprintf(b, "- Memory: %s\n", limits.memory)
	b.WriteString("\nThese are configurable, not hard caps - if a task needs more, tell the user why so they can raise pids_limit/cpus/memory in .agenticrc.toml (or the --pids-limit/--cpus/--memory flags) and rerun.\n\n")
}

func writePrivilegeSection(b *strings.Builder) {
	b.WriteString("## Privileges\n\n")
	b.WriteString("- Running as a non-root user with all Linux capabilities dropped and no privilege escalation.\n")
	b.WriteString("- `sudo`, system package installs, and binding privileged ports will not work.\n\n")
}

// writeNetworkSection is only written when the proxy is enabled; the allowlist itself is omitted in monitor mode, since nothing is actually blocked there.
func writeNetworkSection(b *strings.Builder, toolConfig tools.ToolConfig, rc *config.AgenticRC, in Input) {
	if !in.ProxyEnabled {
		return
	}

	b.WriteString("## Network\n\n")
	b.WriteString("- All network egress routes through an egress proxy - there is no direct internet access.\n")

	if in.ProxyMonitor {
		b.WriteString("\n")
		return
	}

	b.WriteString("- Only the following hosts are reachable:\n")
	for _, host := range proxyAllowList(toolConfig, rc) {
		fmt.Fprintf(b, "  - %s\n", host)
	}
	b.WriteString("\nAnything not listed above is blocked. If a task needs a host that isn't reachable, tell the user why so they can add it to allowed_hosts in .agenticrc.toml and rerun.\n\n")
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
