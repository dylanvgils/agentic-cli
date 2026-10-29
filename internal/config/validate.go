package config

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// validRCEntryName matches safe path-segment/identifier characters, used for both marketplace and custom-install names.
var validRCEntryName = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// scpGitURL matches an scp-style [user@]host:path URL; the host needs two characters so a Windows drive isn't one, and
// the path can't start with ':' so ext::/fd:: transports aren't either.
var scpGitURL = regexp.MustCompile(`^([^@/\s]+@)?[^@/:\s]{2,}:[^:\s]\S*$`)

// validMemory matches a docker memory size: a number with an optional b, k, m or g unit.
var validMemory = regexp.MustCompile(`^[0-9]+[bkmgBKMG]?$`)

// validateRC checks one layer's values; the caller prefixes the error with the layer's path.
func validateRC(rc *AgenticRC) error {
	if err := validateRun(rc.Run); err != nil {
		return err
	}

	for _, marketplace := range rc.Marketplaces {
		if err := validateMarketplace(marketplace); err != nil {
			return err
		}
	}

	return validateCustomInstalls(rc.Build.CustomInstalls)
}

// validateRun checks the [run] table and the tables under it.
func validateRun(run RCRun) error {
	if run.Runtime != "" {
		if _, err := sandbox.ParseRuntime(run.Runtime); err != nil {
			return fmt.Errorf("[run] %w", err)
		}
	}

	if err := validateLimits(run.RCLimits); err != nil {
		return fmt.Errorf("[run] %w", err)
	}
	if err := validateLimits(run.Dind.RCLimits); err != nil {
		return fmt.Errorf("[run.dind] %w", err)
	}

	return validateProxy(run.Proxy)
}

// validateProxy checks the [run.proxy] table.
func validateProxy(proxy RCProxy) error {
	if proxy.Mode != "" && proxy.Mode != ModeEnforce && proxy.Mode != ModeMonitor {
		return fmt.Errorf("invalid [run.proxy] mode %q: must be %q or %q", proxy.Mode, ModeEnforce, ModeMonitor)
	}

	for i, cred := range proxy.Credentials {
		if err := validateCredential(cred); err != nil {
			return fmt.Errorf("[[run.proxy.credentials]] entry %d: %w", i+1, err)
		}
	}

	for i, rule := range proxy.AllowedPaths {
		if err := validateAllowedPath(rule); err != nil {
			return fmt.Errorf("[[run.proxy.allowed_paths]] entry %d: %w", i+1, err)
		}
	}

	if err := sandbox.CheckProxyIntegrations(proxy.Integrations); err != nil {
		return fmt.Errorf("[run.proxy] integrations: %w", err)
	}
	return nil
}

// validateCredential checks the entry's shape and characters; hosts, headers and presets are checked when it is resolved.
func validateCredential(cred RCCredential) error {
	if cred.Secret == "" {
		return fmt.Errorf("secret must not be empty")
	}

	custom := len(cred.Hosts) > 0 || cred.Header != ""
	if cred.Preset != "" && custom {
		return fmt.Errorf("preset %q cannot be combined with hosts or header", cred.Preset)
	}
	if cred.Preset == "" && (len(cred.Hosts) == 0 || cred.Header == "") {
		return fmt.Errorf("set either preset or both hosts and header")
	}

	// Escape sequences could disguise the entry in the approval prompt
	fields := append([]string{cred.Preset, cred.Header, cred.Secret}, cred.Hosts...)
	for _, field := range append(fields, cred.Env...) {
		if strings.ContainsFunc(field, func(r rune) bool { return !unicode.IsPrint(r) }) {
			return fmt.Errorf("value %q contains control or non-printable characters", field)
		}
	}
	return nil
}

// validateAllowedPath checks the rule's host is a single exact name and each of its paths is a valid pattern; the proxy
// checks the host's characters when it loads the rules.
func validateAllowedPath(rule RCAllowedPath) error {
	host := rule.Host
	if host == "" {
		return fmt.Errorf("host must not be empty")
	}
	if strings.HasPrefix(host, ".") || strings.ContainsAny(host, "*/:,") || strings.ContainsFunc(host, func(r rune) bool { return unicode.IsSpace(r) || !unicode.IsPrint(r) }) {
		return fmt.Errorf("host %q must be an exact hostname, without wildcards, port or path", host)
	}

	if len(rule.Paths) == 0 {
		return fmt.Errorf("host %q: paths must not be empty", host)
	}
	for _, pattern := range rule.Paths {
		if err := ValidatePathPattern(pattern); err != nil {
			return fmt.Errorf("host %q: %w", host, err)
		}
	}
	return nil
}

// validateLimits refuses limits that are malformed or zero or below, which docker reads as no limit, so a file the agent can
// edit can't lift them without a prompt.
func validateLimits(limits RCLimits) error {
	if limits.PidsLimit != "" {
		if n, err := strconv.Atoi(limits.PidsLimit); err != nil || n <= 0 {
			return fmt.Errorf("pids_limit %q: must be a positive whole number", limits.PidsLimit)
		}
	}
	if limits.CPUs != "" {
		if n, err := strconv.ParseFloat(limits.CPUs, 64); err != nil || math.IsNaN(n) || n <= 0 || math.IsInf(n, 0) {
			return fmt.Errorf("cpus %q: must be a positive number", limits.CPUs)
		}
	}
	if limits.Memory != "" {
		if !validMemory.MatchString(limits.Memory) || strings.Trim(limits.Memory, "0bkmgBKMG") == "" {
			return fmt.Errorf("memory %q: must be a positive size, e.g. \"8g\"", limits.Memory)
		}
	}
	return nil
}

func validateMarketplace(marketplace RCMarketplace) error {
	if err := validateEntryName("marketplace", marketplace.Name); err != nil {
		return err
	}
	if marketplace.URL == "" {
		return fmt.Errorf("marketplace %q: url must not be empty", marketplace.Name)
	}
	if !isRemoteGitURL(marketplace.URL) {
		return fmt.Errorf("marketplace %q: url %q must be https://, ssh:// or user@host:path", marketplace.Name, marketplace.URL)
	}
	return nil
}

// isRemoteGitURL reports whether url reaches git over https or ssh, not a host path or another transport.
func isRemoteGitURL(url string) bool {
	if strings.ContainsAny(url, " \t\n") {
		return false
	}
	if strings.Contains(url, "://") {
		return strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "ssh://")
	}
	return scpGitURL.MatchString(url)
}

// validateCustomInstalls checks each [[build.custom_installs]] entry and that names are unique within the layer.
func validateCustomInstalls(installs []RCCustomInstall) error {
	for _, install := range installs {
		if err := validateCustomInstall(install); err != nil {
			return err
		}
	}

	if name, ok := duplicateInstall(installs); ok {
		return fmt.Errorf("custom install %q: name must be unique", name)
	}
	return nil
}

func validateCustomInstall(install RCCustomInstall) error {
	if err := validateEntryName("custom install", install.Name); err != nil {
		return err
	}
	if len(install.Run) == 0 {
		return fmt.Errorf("custom install %q: run must not be empty", install.Name)
	}
	return nil
}

// validateEntryName checks a marketplace or custom install name is set and safe; kind names the entry in the error.
func validateEntryName(kind, name string) error {
	if name == "" {
		return fmt.Errorf("%s: name must not be empty", kind)
	}
	if !validRCEntryName.MatchString(name) {
		return fmt.Errorf("%s %q: name must match %s", kind, name, validRCEntryName.String())
	}
	return nil
}

// duplicateInstall returns the first custom install name that appears twice.
func duplicateInstall(installs []RCCustomInstall) (string, bool) {
	seen := make(map[string]bool, len(installs))
	for _, install := range installs {
		if seen[install.Name] {
			return install.Name, true
		}
		seen[install.Name] = true
	}
	return "", false
}
