package docker

import (
	"fmt"
	"os"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/platform"
)

// RunSpec collects everything needed to run a container.
type RunSpec struct {
	Image          string
	ToolHome       string
	ContainerHome  string
	Volumes        []string
	Secrets        []string
	SkipEntrypoint bool
	TmpfsMounts    []string
	PidsLimit      string
	CPUs           string
	Memory         string
	DryRun         bool
}

const (
	DefaultPidsLimit = "1024"
	DefaultCPUs      = "4"
	DefaultMemory    = "4g"
)

func RunContainer(rs RunSpec, toolArgs []string) error {
	args := buildBaseArgs(rs)

	// Interactive TTY only when stdin is a terminal
	if platform.IsTerminal() {
		args = append(args, "-it")
	}

	args = append(args, buildTmpfsArgs(rs)...)
	args = append(args, buildVolumeArgs(rs)...)

	secretArgs, err := buildSecretArgs(rs)
	if err != nil {
		return err
	}
	args = append(args, secretArgs...)

	if rs.SkipEntrypoint {
		args = append(args, arg("entrypoint", ""))
	}

	args = append(args, rs.Image)
	args = append(args, toolArgs...)

	if rs.DryRun {
		_, err := fmt.Fprintln(os.Stdout, "docker", shellJoin(args))
		return err
	}
	return runInteractive(args...)
}

// resolveLimit returns val if non-empty, then the env var, then fallback.
// Mirrors the bash ${VAR:-default} pattern used in bin/agentic.
func resolveLimit(val, envKey, fallback string) string {
	if val != "" {
		return val
	}
	if env := os.Getenv(envKey); env != "" {
		return env
	}
	return fallback
}

func shellJoin(args []string) string {
	parts := make([]string, len(args))
	for i, arg := range args {
		if strings.ContainsAny(arg, " \t$") {
			parts[i] = "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
		} else {
			parts[i] = arg
		}
	}
	return strings.Join(parts, " ")
}

// buildBaseArgs builds the mandatory security and resource-limit args.
// Base arguments for running the docker container; the goal is to run
// the container with minimal permissions.
func buildBaseArgs(rs RunSpec) []string {
	return []string{
		// Run container read-only, remove when done
		"run", "--rm", "--read-only",
		// Limit the number of PIDs (processes) the container can spawn
		arg("pids-limit", resolveLimit(rs.PidsLimit, "AGENTIC_PIDS_LIMIT", DefaultPidsLimit)),
		// Maximum number of CPUs the container can utilize
		arg("cpus", resolveLimit(rs.CPUs, "AGENTIC_CPUS", DefaultCPUs)),
		// Maximum memory the container can use
		arg("memory", resolveLimit(rs.Memory, "AGENTIC_MEMORY", DefaultMemory)),
		// Security: drop all capabilities
		arg("cap-drop", "ALL"),
		// Security: prevent privilege escalation
		arg("security-opt", "no-new-privileges:true"),
		// Use system user to prevent permission issues on mounted files
		arg("user", platform.UserGroup()),
	}
}

// buildTmpfsArgs builds --tmpfs flags with variable expansion.
func buildTmpfsArgs(rs RunSpec) []string {
	args := make([]string, 0, len(rs.TmpfsMounts))
	for _, t := range rs.TmpfsMounts {
		expanded := mount.ExpandMountSpec(t, rs.ToolHome, rs.ContainerHome)
		args = append(args, arg("tmpfs", expanded))
	}
	return args
}

// buildVolumeArgs builds --volume flags with variable expansion.
func buildVolumeArgs(rs RunSpec) []string {
	args := make([]string, 0, len(rs.Volumes))
	for _, volume := range rs.Volumes {
		expanded := mount.ExpandMountSpec(volume, rs.ToolHome, rs.ContainerHome)
		args = append(args, arg("volume", mount.NormalizeMountSpec(expanded)))
	}
	return args
}

// buildSecretArgs builds read-only secret volume flags.
// Returns an error for any malformed "name:/path" entry.
func buildSecretArgs(rs RunSpec) ([]string, error) {
	args := make([]string, 0, len(rs.Secrets))
	for _, secret := range rs.Secrets {
		name, hostPath, ok := strings.Cut(secret, ":")
		if !ok {
			return nil, fmt.Errorf("invalid secret %q: expected name:/path", secret)
		}

		hostPath = mount.ExpandMountSpec(hostPath, rs.ToolHome, rs.ContainerHome)
		hostPath = mount.NormalizeMountSpec(hostPath)

		args = append(args, arg("volume", mount.VolumeMount(hostPath, "/run/secrets/"+name, mount.VolumeOptions{ReadOnly: true})))
	}
	return args, nil
}
