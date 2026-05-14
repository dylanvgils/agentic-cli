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
	Volumes        []string
	SkipEntrypoint bool
	Spec           config.RunSpec
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

func RunContainer(rs RunSpec, toolArgs []string) error {
	// Base arguments for running the docker container
	// the goal is to run the container with minimal
	// permissions
	args := []string{
		// Run container, remove when done
		"run", "--rm",
		// Read-only file system
		"--read-only",
		// Security: drop all capabilities
		"--cap-drop=ALL",
		// Security: prevent privilege escalation
		"--security-opt=no-new-privileges:true",
		// Use system user to prevent permission issues on mounted files
		"--user", platform.UserGroup(),
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
		args = append(args, "-v", ExpandMountVars(v, rs.ToolHome, ""))
	}

	if rs.SkipEntrypoint {
		args = append(args, "--entrypoint", "")
	}

	args = append(args, rs.Image)
	args = append(args, toolArgs...)
	return runInteractive(args...)
}
