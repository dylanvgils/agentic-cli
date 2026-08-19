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

// InstructionsSnapshot is the per-run view of a tool's global instructions
// file: MountSpec (empty when instructions are disabled) overlays this run's
// generated content on top of the tool's persistent instructions file, so
// concurrent runs of the same tool never share one live, mutable file for the
// whole container session. Cleanup must be deferred by the caller regardless
// of how the run ends, to sync any organic edits back to the persistent file
// and remove the temporary snapshot.
type InstructionsSnapshot struct {
	MountSpec string
	finalize  func() error
}

// Cleanup finalizes the snapshot, warning (not failing) on error since it
// typically runs during unwind via a deferred call right after PrepareInstructions.
func (s InstructionsSnapshot) Cleanup() {
	if s.finalize == nil {
		return
	}
	if err := s.finalize(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not save instructions: %v\n", err)
	}
}

// PrepareInstructions stages this run's environment-instructions content into
// a private per-run snapshot file, returning a mount spec that overlays it
// onto the tool's instructions path inside the container. When content is
// empty (instructions disabled via config), it instead strips any stale
// managed block from the persistent file directly and returns a zero-value
// snapshot (no mount, no-op Cleanup).
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

// BuildInstructions assembles the environment-instructions Markdown written into
// each tool's global instructions file: what's installed (capabilities, read
// from the image's labels so it reflects the actual built image rather than a
// possibly-stale .agenticrc.toml) and what's restricted (filesystem, resource
// limits, privileges, network), plus any user-authored custom text. Returns ""
// when the block is disabled via [run.instructions]. Side-effect free - safe to
// call for a preview (agentic instructions <tool>) without starting a container.
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

// PreviewInstructions returns the effective content a run would write into
// the tool's global instructions file - the generated block merged with
// whatever's already persisted at its host path - without touching any
// files. Used by `agentic instructions <tool>` to preview before running.
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

// writePrecedenceSection scopes this generated section - not the file as a
// whole, which may also carry the user's own notes merged in below it - to
// environment facts (what's installed, what's restricted) rather than coding
// conventions, so it's clear a project's own instructions file governs how to
// work, not whether an operation is possible, and that the user's own
// content is never demoted alongside it.
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

// writeSubList writes label as a bullet followed by each item as a nested
// bullet, e.g. "- Label:\n  - item1\n  - item2\n" - used for toolchain lists
// that can otherwise grow into a hard-to-scan comma run.
func writeSubList(b *strings.Builder, label string, items []string) {
	fmt.Fprintf(b, "- %s:\n", label)
	for _, item := range items {
		fmt.Fprintf(b, "  - %s\n", item)
	}
}

// writeMissingToolsNote states plainly that anything outside the listed
// toolchains cannot be installed at runtime, and points at the one way to
// actually get a missing tool: telling the user why it's needed so they can
// add it and rebuild the image, rather than the agent silently trying and
// failing. Set apart from the bullet list above it since it's a rule, not an
// inventory entry.
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

// tmpfsPath extracts the container-side path from a tmpfs mount spec
// (path[:options]), expanding $CONTAINER_HOME so the text reads as a real path.
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

// writeNetworkSection is only written when the egress proxy is enabled - when
// it's off there is no restriction worth commenting on. The "no direct internet
// access" line holds in both proxy sub-modes, since the container is confined to
// the internal network either way; the allowlist itself is only listed when
// actually enforced; in monitor mode nothing is blocked, so listing hosts there
// would misrepresent it as a real restriction.
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
