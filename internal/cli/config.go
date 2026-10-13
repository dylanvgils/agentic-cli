package cli

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show the effective agentic configuration",
	Long:  `Show the merged configuration from agentic.json and all .agenticrc.toml files.`,
	Args:  cobra.NoArgs,
	RunE:  showConfig,
}

// entry is one shown value and its source: a layer path, or "" for a default.
type entry struct {
	value  string
	source string
}

// field is one config key and its resolved entries.
type field struct {
	key     string
	list    bool
	entries []entry
}

// section is a TOML table and its fields; the top-level section has no header.
type section struct {
	header string
	fields []field
}

// value returns the field's first value, or "" when it has none.
func (f field) value() string {
	if len(f.entries) == 0 {
		return ""
	}
	return f.entries[0].value
}

func init() {
	rootCmd.AddCommand(configCmd)
}

func showConfig(cmd *cobra.Command, _ []string) error {
	cliConfig, err := config.LoadConfig(toolHome)
	if err != nil {
		return err
	}

	state, err := config.LoadState(toolHome)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	layers, err := config.FindLayers(cwd)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	if err := printGlobalConfig(w, toolHome, cliConfig, state); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	return printProjectConfig(w, layers)
}

func printGlobalConfig(w io.Writer, home string, cfg *config.CliConfig, state *config.State) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Global (%s)\n", config.ConfigFile(home))
	fmt.Fprintf(&b, "  registry: %s\n", orNotSet(cfg.Registry))
	fmt.Fprintf(&b, "  docker_context: %s\n", orNotSet(cfg.DockerContext))

	stateFile := config.StateFile(home)
	if len(state.TrustedDirs) == 0 {
		fmt.Fprintf(&b, "  trusted_dirs: (none)  [%s]\n", stateFile)
	} else {
		fmt.Fprintf(&b, "  trusted_dirs:  [%s]\n", stateFile)
		for _, dir := range state.TrustedDirs {
			fmt.Fprintf(&b, "    - %s\n", dir)
		}
	}

	_, err := io.WriteString(w, b.String())
	return err
}

func printProjectConfig(w io.Writer, layers []config.RCLayer) error {
	if len(layers) == 0 {
		_, err := io.WriteString(w, "Project (.agenticrc.toml)\n  (no .agenticrc.toml files found)\n")
		return err
	}

	noun := "file"
	if len(layers) > 1 {
		noun = "files"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Project (.agenticrc.toml, %d %s)\n", len(layers), noun)
	for _, s := range projectSections(layers) {
		writeSection(&b, s)
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// projectSections resolves every project key against layers, grouped by TOML table.
func projectSections(layers []config.RCLayer) []section {
	versions := resolveEffectiveVersions(layers)
	runLimits := limitFields(layers, func(rc *config.AgenticRC) config.RCLimits { return rc.Run.RCLimits }, sandbox.DefaultLimits)

	// Unset sidecar limits inherit the tool's
	dindDefaults := sandbox.ResourceLimits{PidsLimit: runLimits[0].value(), CPUs: runLimits[1].value(), Memory: runLimits[2].value()}
	dindLimits := limitFields(layers, func(rc *config.AgenticRC) config.RCLimits { return rc.Run.Dind.RCLimits }, dindDefaults)

	return []section{
		{fields: []field{
			scalarField(layers, "namespace", func(rc *config.AgenticRC) string { return rc.Namespace }, config.DefaultNamespace),
			scalarField(layers, "docker_context", func(rc *config.AgenticRC) string { return rc.DockerContext }, ""),
			listField(layers, "marketplaces", marketplaceNames),
		}},
		{header: "build", fields: []field{
			listField(layers, "bases", func(rc *config.AgenticRC) []string { return baseNames(rc, versions) }),
			listField(layers, "apt_packages", func(rc *config.AgenticRC) []string { return rc.Build.AptPackages }),
			listField(layers, "custom_installs", customInstallNames),
		}},
		{header: "run", fields: append(runLimits,
			listField(layers, "mounts", func(rc *config.AgenticRC) []string { return rc.Run.Mounts }),
			listField(layers, "secrets", func(rc *config.AgenticRC) []string { return rc.Run.Secrets }),
			listField(layers, "env", envKeys),
			boolField(layers, "check_updates", func(rc *config.AgenticRC) *bool { return rc.Run.CheckUpdates }, true),
		)},
		{header: "run.instructions", fields: []field{
			boolField(layers, "enabled", func(rc *config.AgenticRC) *bool { return rc.Run.Instructions.Enabled }, true),
			listField(layers, "custom", customInstructions),
		}},
		{header: "run.proxy", fields: []field{
			boolField(layers, "enabled", func(rc *config.AgenticRC) *bool { return rc.Run.Proxy.Enabled }, proxyEnabledDefault(layers)),
			scalarField(layers, "mode", func(rc *config.AgenticRC) string { return rc.Run.Proxy.Mode }, config.ModeEnforce),
			listField(layers, "allowed_hosts", func(rc *config.AgenticRC) []string { return rc.Run.Proxy.AllowedHosts }),
			listField(layers, "credentials", credentialNames),
		}},
		{header: "run.dind", fields: append([]field{
			boolField(layers, "enabled", func(rc *config.AgenticRC) *bool { return rc.Run.Dind.Enabled }, false),
		}, dindLimits...)},
	}
}

// proxyEnabledDefault reports whether the proxy runs when no layer sets enabled: monitor mode or credentials turn it on.
func proxyEnabledDefault(layers []config.RCLayer) bool {
	mode, _, _ := innermost(layers, func(rc *config.AgenticRC) string { return rc.Run.Proxy.Mode }, func(v string) bool { return v != "" })
	if mode == config.ModeMonitor {
		return true
	}

	return slices.ContainsFunc(layers, func(l config.RCLayer) bool { return len(l.RC.Run.Proxy.Credentials) > 0 })
}

// limitFields returns the pids_limit, cpus and memory fields of the limits get picks, falling back to def.
func limitFields(layers []config.RCLayer, get func(*config.AgenticRC) config.RCLimits, def sandbox.ResourceLimits) []field {
	return []field{
		scalarField(layers, "pids_limit", func(rc *config.AgenticRC) string { return get(rc).PidsLimit }, def.PidsLimit),
		scalarField(layers, "cpus", func(rc *config.AgenticRC) string { return get(rc).CPUs }, def.CPUs),
		scalarField(layers, "memory", func(rc *config.AgenticRC) string { return get(rc).Memory }, def.Memory),
	}
}

// scalarField resolves a string key: innermost layer wins, else def, else no entry.
func scalarField(layers []config.RCLayer, key string, get func(*config.AgenticRC) string, def string) field {
	isSet := func(v string) bool { return v != "" }
	if v, path, ok := innermost(layers, get, isSet); ok {
		return field{key: key, entries: []entry{{value: v, source: path}}}
	}

	if def != "" {
		return field{key: key, entries: []entry{{value: def}}}
	}

	return field{key: key}
}

// boolField resolves a bool key: innermost layer that sets it wins, else def.
func boolField(layers []config.RCLayer, key string, get func(*config.AgenticRC) *bool, def bool) field {
	isSet := func(v *bool) bool { return v != nil }
	if v, path, ok := innermost(layers, get, isSet); ok {
		return field{key: key, entries: []entry{{value: strconv.FormatBool(*v), source: path}}}
	}

	return field{key: key, entries: []entry{{value: strconv.FormatBool(def)}}}
}

// listField resolves a list key: every layer's values, outermost first.
func listField(layers []config.RCLayer, key string, get func(*config.AgenticRC) []string) field {
	f := field{key: key, list: true}
	for _, layer := range layers {
		for _, v := range get(layer.RC) {
			f.entries = append(f.entries, entry{value: v, source: layer.Path})
		}
	}

	return f
}

// innermost returns the value from the innermost layer where isSet holds, and that layer's path.
func innermost[T any](layers []config.RCLayer, get func(*config.AgenticRC) T, isSet func(T) bool) (T, string, bool) {
	for i := len(layers) - 1; i >= 0; i-- {
		if v := get(layers[i].RC); isSet(v) {
			return v, layers[i].Path, true
		}
	}

	var zero T
	return zero, "", false
}

// writeSection writes s's header, if any, and its fields indented below it.
func writeSection(b *strings.Builder, s section) {
	indent := "  "
	if s.header != "" {
		fmt.Fprintf(b, "\n  [%s]\n", s.header)
		indent = "    "
	}

	for _, f := range s.fields {
		writeField(b, f, indent)
	}
}

// writeField writes one key with its values, each tagged with its source layer or (default).
func writeField(b *strings.Builder, f field, indent string) {
	switch {
	case len(f.entries) == 0 && f.list:
		fmt.Fprintf(b, "%s%s: (none)\n", indent, f.key)
	case len(f.entries) == 0:
		fmt.Fprintf(b, "%s%s: (not set)\n", indent, f.key)
	case f.list:
		fmt.Fprintf(b, "%s%s:\n", indent, f.key)
		for _, e := range f.entries {
			fmt.Fprintf(b, "%s  - %s%s\n", indent, e.value, sourceTag(e.source))
		}
	default:
		fmt.Fprintf(b, "%s%s: %s%s\n", indent, f.key, f.entries[0].value, sourceTag(f.entries[0].source))
	}
}

// sourceTag returns the suffix naming where a value came from.
func sourceTag(source string) string {
	if source == "" {
		return "  (default)"
	}
	return "  [" + source + "]"
}

// baseNames returns rc's bases, with their effective version inlined as name@version.
func baseNames(rc *config.AgenticRC, versions map[string]string) []string {
	names := make([]string, len(rc.Build.Bases))
	for i, name := range rc.Build.Bases {
		if v, ok := versions[name]; ok {
			names[i] = name + "@" + v
		} else {
			names[i] = name
		}
	}
	return names
}

// customInstallNames returns the names of rc's custom installs.
func customInstallNames(rc *config.AgenticRC) []string {
	names := make([]string, len(rc.Build.CustomInstalls))
	for i, ci := range rc.Build.CustomInstalls {
		names[i] = ci.Name
	}
	return names
}

// envKeys returns the keys of rc's env entries, leaving out values that may hold secrets.
func envKeys(rc *config.AgenticRC) []string {
	keys := make([]string, len(rc.Run.Env))
	for i, env := range rc.Run.Env {
		keys[i], _, _ = strings.Cut(env, "=")
	}
	return keys
}

// credentialNames returns each of rc's credentials as its preset, or its hosts when it has none.
func credentialNames(rc *config.AgenticRC) []string {
	names := make([]string, len(rc.Run.Proxy.Credentials))
	for i, cred := range rc.Run.Proxy.Credentials {
		names[i] = cred.Preset
		if names[i] == "" {
			names[i] = strings.Join(cred.Hosts, ", ")
		}
	}
	return names
}

// marketplaceNames returns each of rc's marketplaces as "name (url)", plus the tools it is limited to.
func marketplaceNames(rc *config.AgenticRC) []string {
	names := make([]string, len(rc.Marketplaces))
	for i, m := range rc.Marketplaces {
		names[i] = fmt.Sprintf("%s (%s)", m.Name, m.URL)
		if len(m.Tools) > 0 {
			names[i] += " tools: " + strings.Join(m.Tools, ", ")
		}
	}
	return names
}

// customInstructions summarizes rc's custom instructions by line count, since the text can be long.
func customInstructions(rc *config.AgenticRC) []string {
	custom := strings.TrimSpace(rc.Run.Instructions.Custom)
	if custom == "" {
		return nil
	}

	lines := strings.Count(custom, "\n") + 1
	if lines == 1 {
		return []string{"(1 line)"}
	}
	return []string{fmt.Sprintf("(%d lines)", lines)}
}

// orNotSet returns v, or "(not set)" when it is empty.
func orNotSet(v string) string {
	if v == "" {
		return "(not set)"
	}
	return v
}

// resolveEffectiveVersions builds the version map for bases: innermost RC layer wins per key.
func resolveEffectiveVersions(layers []config.RCLayer) map[string]string {
	versions := map[string]string{}

	for _, layer := range layers {
		for name, v := range layer.RC.Build.Versions {
			if v != "" {
				versions[name] = v
			}
		}
	}

	return versions
}
