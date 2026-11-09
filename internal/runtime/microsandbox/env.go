package microsandbox

import (
	"fmt"
	"os"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

const (
	// dryRunEnvConfPlaceholder stands in for the forwarded env file in dry-run output, as it isn't written.
	dryRunEnvConfPlaceholder = "<forwarded env file>"

	// guestEnvPrefix renames guest-only values in msb's env, so they can't steer msb's own HTTP calls.
	guestEnvPrefix = "AGENTIC_GUEST_"
)

// envRef maps a guest env key to the msb env var msb reads its value from.
type envRef struct {
	guest  string
	source string
}

// buildEnvArgs returns the --env flags plus a --runtime-conf file for forwarded vars and hidden entries, kept out of args.
func buildEnvArgs(rs sandbox.RunSpec, hidden []string) (args []string, cleanup func(), err error) {
	flags, forwarded := splitEnv(sandbox.ResolveEnv(rs.Env))
	refs := make([]envRef, 0, len(forwarded)+len(hidden))
	for _, key := range forwarded {
		refs = append(refs, envRef{guest: key, source: key})
	}
	for _, entry := range hidden {
		key, _, _ := strings.Cut(entry, "=")
		refs = append(refs, envRef{guest: key, source: guestEnvName(key)})
	}
	if len(refs) == 0 {
		return flags, noCleanup, nil
	}

	if rs.DryRun {
		return append(flags, arg("runtime-conf", dryRunEnvConfPlaceholder)), noCleanup, nil
	}

	path, err := writeEnvConf(refs)
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

// guestProcessEnv renames hidden guest entries for msb's own env, matching the refs buildEnvArgs writes.
func guestProcessEnv(hidden []string) []string {
	env := make([]string, 0, len(hidden))
	for _, entry := range hidden {
		key, value, _ := strings.Cut(entry, "=")
		env = append(env, guestEnvName(key)+"="+value)
	}
	return env
}

// guestEnvName returns the private msb env name holding a guest-only value for key.
func guestEnvName(key string) string {
	return guestEnvPrefix + key
}

// writeEnvConf writes a temp --runtime-conf file mapping each guest key to a ${source} reference, which msb resolves
// from its own env, so the file holds no values.
func writeEnvConf(refs []envRef) (string, error) {
	var b strings.Builder
	b.WriteString("env:\n")
	for _, ref := range refs {
		fmt.Fprintf(&b, "  %q: \"${%s}\"\n", ref.guest, ref.source)
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
