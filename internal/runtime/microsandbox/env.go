package microsandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

const (
	// dryRunEnvConfPlaceholder stands in for the forwarded env file in dry-run output, as it isn't written.
	dryRunEnvConfPlaceholder = "<forwarded env file>"

	// guestEnvPrefix renames guest-only values in msb's env, so msb itself ignores them.
	guestEnvPrefix = "AGENTIC_GUEST_"
)

// envRef maps a guest key to the msb env var holding its value.
type envRef struct {
	guest  string
	source string
}

// buildEnvArgs returns the --env flags plus a --runtime-conf file for forwarded vars and hidden entries, kept out of args.
func buildEnvArgs(rs sandbox.RunSpec, hidden []string) ([]string, error) {
	flags, refs := splitEnv(sandbox.ResolveEnv(rs.Env))
	for _, entry := range hidden {
		key, _, _ := strings.Cut(entry, "=")
		refs = append(refs, envRef{guest: key, source: guestEnvName(key)})
	}
	if len(refs) == 0 {
		return flags, nil
	}

	if rs.DryRun {
		return append(flags, arg("runtime-conf", dryRunEnvConfPlaceholder)), nil
	}

	path, err := writeEnvConf(rs.RunID, refs)
	if err != nil {
		return nil, err
	}
	return append(flags, arg("runtime-conf", path)), nil
}

// splitEnv splits resolved env into --env flags and the forwarded bare keys msb's --env can't take; a key's last
// entry wins. A non-portable bare key gets its host value as a flag, as msb only substitutes portable names.
func splitEnv(resolved []string) (flags []string, forwarded []envRef) {
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
			forwarded = append(forwarded, envRef{guest: key, source: key})
		default:
			flags = append(flags, arg("env", key+"="+os.Getenv(key)))
		}
	}
	return flags, forwarded
}

// guestProcessEnv renames hidden entries for msb's env.
func guestProcessEnv(hidden []string) []string {
	env := make([]string, 0, len(hidden))
	for _, entry := range hidden {
		key, value, _ := strings.Cut(entry, "=")
		env = append(env, guestEnvName(key)+"="+value)
	}
	return env
}

// guestEnvName returns the private msb env name for key.
func guestEnvName(key string) string {
	return guestEnvPrefix + key
}

// writeEnvConf writes run id's --runtime-conf file mapping each key to a ${source} reference, so it holds no values.
func writeEnvConf(id sandbox.RunID, refs []envRef) (string, error) {
	var b strings.Builder
	b.WriteString("env:\n")
	for _, ref := range refs {
		fmt.Fprintf(&b, "  %q: \"${%s}\"\n", ref.guest, ref.source)
	}

	f, err := os.OpenFile(envConfPath(id), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
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

// envConfPath returns run id's --runtime-conf file.
func envConfPath(id sandbox.RunID) string {
	return filepath.Join(os.TempDir(), "agentic-msb-env-"+id.String()+".yaml")
}
