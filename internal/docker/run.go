package docker

import (
	"os"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/platform"
)

// RunSpec collects everything needed to run a container.
type RunSpec struct {
	Image          string
	ToolHome       string
	ContainerHome  string
	Volumes        []string
	SkipEntrypoint bool
	Spec           config.RunSpec
	PidsLimit      string
	CPUs           string
	Memory         string
}

// ExpandMountVars replaces $TOOL_HOME, ${TOOL_HOME}, $CONTAINER_HOME, ${CONTAINER_HOME},
// and $PWD in a mount spec string.
func ExpandMountVars(spec, toolHome, containerHome string) string {
	pwd, _ := os.Getwd()
	s := spec
	s = strings.ReplaceAll(s, "${CONTAINER_HOME}", containerHome)
	s = strings.ReplaceAll(s, "$CONTAINER_HOME", containerHome)
	s = strings.ReplaceAll(s, "${TOOL_HOME}", toolHome)
	s = strings.ReplaceAll(s, "$TOOL_HOME", toolHome)
	s = strings.ReplaceAll(s, "$PWD", pwd)
	return s
}

var runInteractive = RunInteractive

// arg builds a --name=value Docker flag.
// Panics if name is empty or starts with '-' (programmer error).
func arg(name, value string) string {
	if name == "" {
		panic("docker: arg name must not be empty")
	}
	if strings.HasPrefix(name, "-") {
		panic("docker: arg name must not start with '-', got: " + name)
	}
	return "--" + name + "=" + value
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

	// /tmp tmpfs
	tmpFlags := "size=1g"
	if rs.Spec.TmpfsExecTmp {
		tmpFlags = "exec," + tmpFlags
	}
	args = append(args, "--tmpfs", "/tmp:"+tmpFlags)

	for _, v := range rs.Volumes {
		varg := arg("volume", ExpandMountVars(v, rs.ToolHome, rs.ContainerHome))
		args = append(args, varg)
	}

	if rs.SkipEntrypoint {
		args = append(args, arg("entrypoint", ""))
	}

	args = append(args, rs.Image)
	args = append(args, toolArgs...)
	return runInteractive(args...)
}
