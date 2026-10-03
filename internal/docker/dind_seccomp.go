package docker

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"slices"
)

// dindSeccompFileName is the per-run profile's name inside the run dir.
const dindSeccompFileName = "seccomp.json"

// errnoENOSYS makes a syscall look unimplemented, which callers like runc treat as "feature absent" rather than a hard failure.
const errnoENOSYS = 38

// dindSeccompDefault is Docker's default seccomp profile, vendored verbatim from
// github.com/moby/profiles (seccomp/default.json @ 6fe7deb, 2026-09-17); refresh by re-copying.
//
//go:embed dind_seccomp_default.json
var dindSeccompDefault []byte

// dindSeccompDropped are syscalls the default profile unlocks with CAP_SYS_ADMIN (or other caps)
// that neither rootless dockerd nor typical inner containers need; keeping them blocked keeps
// eBPF, perf, the kernel log and fanotify out of reach.
var dindSeccompDropped = []string{
	"bpf", "fanotify_init", "lookup_dcookie", "perf_event_open", "quotactl", "quotactl_fd", "syslog",
}

// dindSeccompExtraRules are what nested runc needs beyond the default profile: pivot_root to enter
// an inner container's rootfs, and keyring syscalls failing with ENOSYS (which runc tolerates)
// instead of the default EPERM (which it doesn't) - so keyrings stay blocked.
var dindSeccompExtraRules = []map[string]any{
	{
		"names":    []string{"pivot_root"},
		"action":   "SCMP_ACT_ALLOW",
		"includes": map[string]any{"caps": []string{"CAP_SYS_ADMIN"}},
	},
	{
		"names":    []string{"add_key", "keyctl", "request_key"},
		"action":   "SCMP_ACT_ERRNO",
		"errnoRet": errnoENOSYS,
	},
}

// dindSeccompProfile derives the sidecar's profile from Docker's default: dindSeccompDropped
// removed from every allow rule, dindSeccompExtraRules appended, everything else untouched.
func dindSeccompProfile() ([]byte, error) {
	var profile map[string]any
	if err := json.Unmarshal(dindSeccompDefault, &profile); err != nil {
		return nil, fmt.Errorf("parse vendored seccomp profile: %w", err)
	}

	rules, ok := profile["syscalls"].([]any)
	if !ok {
		return nil, fmt.Errorf("vendored seccomp profile has no syscalls list")
	}

	kept := make([]any, 0, len(rules)+len(dindSeccompExtraRules))
	for _, raw := range rules {
		rule, ok := raw.(map[string]any)
		if !ok || rule["action"] != "SCMP_ACT_ALLOW" {
			kept = append(kept, raw)
			continue
		}

		names := withoutDroppedSyscalls(rule["names"])
		if len(names) == 0 {
			continue
		}
		rule["names"] = names
		kept = append(kept, rule)
	}

	for _, rule := range dindSeccompExtraRules {
		kept = append(kept, rule)
	}
	profile["syscalls"] = kept

	return json.MarshalIndent(profile, "", "  ")
}

// writeDindSeccompProfile writes the derived profile to path; the docker CLI reads it client-side, so a host path works with any daemon.
func writeDindSeccompProfile(path string) error {
	content, err := dindSeccompProfile()
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("write seccomp profile: %w", err)
	}
	return nil
}

// withoutDroppedSyscalls returns a rule's names minus dindSeccompDropped.
func withoutDroppedSyscalls(raw any) []any {
	names, _ := raw.([]any)
	result := make([]any, 0, len(names))
	for _, name := range names {
		if s, ok := name.(string); ok && slices.Contains(dindSeccompDropped, s) {
			continue
		}
		result = append(result, name)
	}
	return result
}
