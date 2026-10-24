package microsandbox

import (
	"fmt"
	"os"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// dryRunEnvConfPlaceholder stands in for the forwarded env file in dry-run output, as it isn't written.
const dryRunEnvConfPlaceholder = "<forwarded env file>"

// buildEnvArgs returns the --env flags and, for forwarded host vars, a --runtime-conf file msb resolves them from; cleanup removes it.
func buildEnvArgs(rs sandbox.RunSpec) (args []string, cleanup func(), err error) {
	flags, forwarded := splitEnv(sandbox.ResolveEnv(rs.Env))
	if len(forwarded) == 0 {
		return flags, noCleanup, nil
	}

	if rs.DryRun {
		return append(flags, arg("runtime-conf", dryRunEnvConfPlaceholder)), noCleanup, nil
	}

	path, err := writeEnvConf(forwarded)
	if err != nil {
		return nil, nil, err
	}
	return append(flags, arg("runtime-conf", path)), func() { _ = os.Remove(path) }, nil
}

// splitEnv splits resolved env into --env flags and the forwarded bare keys msb's --env can't take; a key's last
// entry wins. A non-portable bare key gets its host value as a flag, as msb only substitutes portable names.
func splitEnv(resolved []string) (flags, forwarded []string) {
	last := make(map[string]int, len(resolved))
	for i, entry := range resolved {
		key, _, _ := strings.Cut(entry, "=")
		last[key] = i
	}

	for i, entry := range resolved {
		key, _, isPair := strings.Cut(entry, "=")
		switch {
		case last[key] != i:
			continue
		case isPair:
			flags = append(flags, arg("env", entry))
		case sandbox.IsPortableEnvName(key):
			forwarded = append(forwarded, key)
		default:
			flags = append(flags, arg("env", key+"="+os.Getenv(key)))
		}
	}
	return flags, forwarded
}

// writeEnvConf writes a temp --runtime-conf file mapping each key to a ${KEY} reference, which msb resolves from its own
// env, so the file holds no values.
func writeEnvConf(keys []string) (string, error) {
	var b strings.Builder
	b.WriteString("env:\n")
	for _, key := range keys {
		fmt.Fprintf(&b, "  %q: \"${%s}\"\n", key, key)
	}

	f, err := os.CreateTemp("", "agentic-msb-env-*.yaml")
	if err != nil {
		return "", fmt.Errorf("create msb env file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(b.String()); err != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("write msb env file: %w", err)
	}
	return f.Name(), nil
}

// noCleanup is the cleanup for paths that set nothing up.
func noCleanup() {
	// Nothing to undo
}
