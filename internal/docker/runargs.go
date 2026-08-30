package docker

import (
	"fmt"
	"os"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/platform"
)

var (
	// isTerminal is a test-stubbable indirection into platform.IsTerminal.
	isTerminal = platform.IsTerminal
	// hostTimezone is a test-stubbable indirection into platform.Timezone.
	hostTimezone = platform.Timezone
)

// terminalCapabilityEnvNames are host vars auto-forwarded into the container; not reserved, so
// a user-supplied --env entry for one of these can override it (a cosmetic preference, not enforced).
var terminalCapabilityEnvNames = []string{"COLORTERM", "TERM", "NO_COLOR", "FORCE_COLOR"}

// networkOrDefault returns the configured network, or NetworkName when unset.
func networkOrDefault(network string) string {
	if network == "" {
		return NetworkName
	}
	return network
}

// buildBaseArgs builds the mandatory security and resource-limit args for running the container with minimal permissions.
func buildBaseArgs(rs RunSpec) ([]string, error) {
	id, err := randID()
	if err != nil {
		return nil, err
	}

	return []string{
		// Run container read-only, remove when done
		"run", "--rm", "--read-only",
		// Identify the container in `docker ps`/logs; randomized per run for
		arg("name", rs.Image+"-"+id),
		label(LabelProject, LabelProjectVal),
		// Limit the number of PIDs (processes) the container can spawn
		arg("pids-limit", rs.PidsLimit),
		// Maximum number of CPUs the container can utilize
		arg("cpus", rs.CPUs),
		// Maximum memory the container can use
		arg("memory", rs.Memory),
		// Security: isolate from other host containers (proxy mode swaps this
		// for a per-run internal network with no direct egress)
		arg("network", networkOrDefault(rs.network)),
		// Security: drop all capabilities
		arg("cap-drop", "ALL"),
		// Security: prevent privilege escalation
		arg("security-opt", "no-new-privileges:true"),
		// Use system user to prevent permission issues on mounted files
		arg("user", platform.UserGroup()),
	}, nil
}

// buildTTYArgs returns [--interactive --tty] when stdin is a terminal, otherwise empty.
func buildTTYArgs() []string {
	if isTerminal() {
		return []string{arg("interactive"), arg("tty")}
	}
	return nil
}

// buildEnvArgs builds --env flags: auto-forwarded terminal capabilities and host timezone, then
// rs.Env entries ("KEY=VALUE", or bare "KEY" to forward the host's current value).
func buildEnvArgs(rs RunSpec) []string {
	args := forwardEnvArg(terminalCapabilityEnvNames...)

	if tz := hostTimezone(); tz != "" {
		args = append(args, arg("env", "TZ="+tz))
	}

	for _, entry := range rs.Env {
		if key, _, ok := strings.Cut(entry, "="); ok {
			args = append(args, arg("env", entry))
		} else if value, set := os.LookupEnv(key); set {
			args = append(args, arg("env", key+"="+value))
		}
	}

	return args
}

// buildTmpfsArgs builds --tmpfs flags with variable expansion.
func buildTmpfsArgs(rs RunSpec) []string {
	args := make([]string, 0, len(rs.TmpfsMounts))
	for _, t := range rs.TmpfsMounts {
		expanded := mount.ExpandTmpfsSpec(t, rs.ContainerHome)
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

// buildSecretArgs builds read-only secret volume flags, erroring on a malformed "name:/path[:/container/path]" entry.
func buildSecretArgs(rs RunSpec) ([]string, error) {
	args := make([]string, 0, len(rs.Secrets))
	for _, secret := range rs.Secrets {
		name, rest, ok := strings.Cut(secret, ":")
		if !ok {
			return nil, fmt.Errorf("invalid secret %q: expected name:/path[:/container/path]", secret)
		}

		hostPath, containerPath, found := mount.SplitHostContainer(rest)
		if !found {
			containerPath = "/run/secrets/" + name
		} else if containerPath == "" {
			return nil, fmt.Errorf("invalid secret %q: empty container path", secret)
		}

		spec := mount.ExpandMountSpec(hostPath+":"+containerPath, rs.ToolHome, rs.ContainerHome)
		spec = mount.NormalizeMountSpec(spec)
		args = append(args, arg("volume", spec+":ro"))
	}
	return args, nil
}
