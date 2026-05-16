package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
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
	Spec           config.RunSpec
	PidsLimit      string
	CPUs           string
	Memory         string
	DryRun         bool
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

func RunContainer(rs RunSpec, toolArgs []string) error {
	// Base arguments for running the docker container
	// the goal is to run the container with minimal
	// permissions
	args := []string{
		// Run container read-only, remove when done
		"run", "--rm", "--read-only",
		// Limit the number of PIDs (processes) the container can spawn
		arg("pids-limit", resolveLimit(rs.PidsLimit, "AGENTIC_PIDS_LIMIT", "1024")),
		// Maximum number of CPUs the container can utilize
		arg("cpus", resolveLimit(rs.CPUs, "AGENTIC_CPUS", "4")),
		// Maximum memory the container can use
		arg("memory", resolveLimit(rs.Memory, "AGENTIC_MEMORY", "4g")),
		// Security: drop all capabilities
		arg("cap-drop", "ALL"),
		// Security: prevent privilege escalation
		arg("security-opt", "no-new-privileges:true"),
		// Use system user to prevent permission issues on mounted files
		arg("user", platform.UserGroup()),
	}

	// Interactive TTY only when stdin is a terminal
	if platform.IsTerminal() {
		args = append(args, "-it")
	}

	args = append(args, "--tmpfs", mount.TmpfsMount("/tmp", mount.TmpfsOptions{
		Exec: rs.Spec.TmpfsExecTmp,
		Size: "1g",
	}))

	for _, v := range rs.Volumes {
		varg := arg("volume", mount.ExpandVars(v, rs.ToolHome, rs.ContainerHome))
		args = append(args, varg)
	}

	for _, s := range rs.Secrets {
		name, hostPath, ok := strings.Cut(s, "=")
		if !ok {
			return fmt.Errorf("invalid secret %q: expected name=/path", s)
		}
		if strings.HasPrefix(hostPath, "~/") {
			home, _ := os.UserHomeDir()
			hostPath = filepath.Join(home, hostPath[2:])
		}
		args = append(args, arg("volume", mount.VolumeMount(hostPath, "/run/secrets/"+name, mount.VolumeOptions{ReadOnly: true})))
	}

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

func shellJoin(args []string) string {
	parts := make([]string, len(args))
	for i, a := range args {
		if strings.ContainsAny(a, " \t$") {
			parts[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		} else {
			parts[i] = a
		}
	}
	return strings.Join(parts, " ")
}
