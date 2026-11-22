package cli

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config/rc"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
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

func showConfig(_ *cobra.Command, _ []string) error {
	cliConfig, err := rc.LoadConfig(toolHome)
	if err != nil {
		return err
	}

	state, err := rc.LoadState(toolHome)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	layers, err := rc.FindLayers(cwd)
	if err != nil {
		return err
	}

	printGlobalConfig(toolHome, cliConfig, state)
	iostreams.Println()
	printProjectConfig(layers)
	return nil
}

func printGlobalConfig(home string, cfg *rc.CliConfig, state *rc.State) {
	var b strings.Builder
	fmt.Fprintf(&b, "Global (%s)\n", rc.ConfigFile(home))
	fmt.Fprintf(&b, "  registry: %s\n", orNotSet(cfg.Registry))
	fmt.Fprintf(&b, "  docker_context: %s\n", orNotSet(cfg.DockerContext))

	stateFile := rc.StateFile(home)
	if len(state.TrustedDirs) == 0 {
		fmt.Fprintf(&b, "  trusted_dirs: (none)  [%s]\n", stateFile)
	} else {
		fmt.Fprintf(&b, "  trusted_dirs:  [%s]\n", stateFile)
		for _, dir := range state.TrustedDirs {
			fmt.Fprintf(&b, "    - %s\n", dir)
		}
	}

	iostreams.Print(b.String())
}

func printProjectConfig(layers []rc.RCLayer) {
	if len(layers) == 0 {
		iostreams.Println("Project (.agenticrc.toml)")
		iostreams.Println("  (no .agenticrc.toml files found)")
		return
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

	iostreams.Print(b.String())
}

// projectSections resolves every project key against layers, grouped by TOML table.
func projectSections(layers []rc.RCLayer) []section {
	versions := resolveEffectiveVersions(layers)
	runLimits := limitFields(layers, func(agenticRC *rc.AgenticRC) rc.RCLimits { return agenticRC.Run.RCLimits }, sandbox.DefaultLimits)

	// Unset sidecar limits inherit the tool's
	dindDefaults := sandbox.ResourceLimits{PidsLimit: runLimits[0].value(), CPUs: runLimits[1].value(), Memory: runLimits[2].value()}
	dindLimits := limitFields(layers, func(agenticRC *rc.AgenticRC) rc.RCLimits { return agenticRC.Run.Dind.RCLimits }, dindDefaults)

	return []section{
		{fields: []field{
			scalarField(layers, "namespace", func(agenticRC *rc.AgenticRC) string { return agenticRC.Namespace }, rc.DefaultNamespace),
			scalarField(layers, "docker_context", func(agenticRC *rc.AgenticRC) string { return agenticRC.DockerContext }, ""),
			listField(layers, "marketplaces", marketplaceNames),
		}},
		{header: "build", fields: []field{
			listField(layers, "bases", func(agenticRC *rc.AgenticRC) []string { return baseNames(agenticRC, versions) }),
			listField(layers, "apt_packages", func(agenticRC *rc.AgenticRC) []string { return agenticRC.Build.AptPackages }),
			listField(layers, "custom_installs", customInstallNames),
		}},
		{header: "run", fields: append(runLimits,
			listField(layers, "mounts", func(agenticRC *rc.AgenticRC) []string { return agenticRC.Run.Mounts }),
			listField(layers, "env", envKeys),
			listField(layers, "args", toolArgs),
			boolField(layers, "check_updates", func(agenticRC *rc.AgenticRC) *bool { return agenticRC.Run.CheckUpdates }, true),
		)},
		{header: "run.instructions", fields: []field{
			boolField(layers, "enabled", func(agenticRC *rc.AgenticRC) *bool { return agenticRC.Run.Instructions.Enabled }, true),
			listField(layers, "custom", customInstructions),
		}},
		{header: "run.proxy", fields: []field{
			boolField(layers, "enabled", func(agenticRC *rc.AgenticRC) *bool { return agenticRC.Run.Proxy.Enabled }, proxyEnabledDefault(layers)),
			scalarField(layers, "mode", func(agenticRC *rc.AgenticRC) string { return agenticRC.Run.Proxy.Mode }, rc.ModeEnforce),
			listField(layers, "allowed_hosts", func(agenticRC *rc.AgenticRC) []string { return agenticRC.Run.Proxy.AllowedHosts }),
			listField(layers, "allowed_paths", allowedPaths),
			listField(layers, "credentials", credentialNames),
		}},
		{header: "run.dind", fields: append([]field{
			boolField(layers, "enabled", func(agenticRC *rc.AgenticRC) *bool { return agenticRC.Run.Dind.Enabled }, false),
		}, dindLimits...)},
	}
}

// proxyEnabledDefault reports whether the proxy runs when no layer sets enabled: monitor mode or credentials turn it on.
func proxyEnabledDefault(layers []rc.RCLayer) bool {
	mode, _, _ := innermost(layers, func(agenticRC *rc.AgenticRC) string { return agenticRC.Run.Proxy.Mode }, func(v string) bool { return v != "" })
	if mode == rc.ModeMonitor {
		return true
	}

	return slices.ContainsFunc(layers, func(l rc.RCLayer) bool { return len(l.RC.Run.Proxy.Credentials) > 0 })
}

// limitFields returns the pids_limit, cpus and memory fields of the limits get picks, falling back to def.
func limitFields(layers []rc.RCLayer, get func(*rc.AgenticRC) rc.RCLimits, def sandbox.ResourceLimits) []field {
	return []field{
		scalarField(layers, "pids_limit", func(agenticRC *rc.AgenticRC) string { return get(agenticRC).PidsLimit }, def.PidsLimit),
		scalarField(layers, "cpus", func(agenticRC *rc.AgenticRC) string { return get(agenticRC).CPUs }, def.CPUs),
		scalarField(layers, "memory", func(agenticRC *rc.AgenticRC) string { return get(agenticRC).Memory }, def.Memory),
	}
}

// scalarField resolves a string key: innermost layer wins, else def, else no entry.
func scalarField(layers []rc.RCLayer, key string, get func(*rc.AgenticRC) string, def string) field {
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
func boolField(layers []rc.RCLayer, key string, get func(*rc.AgenticRC) *bool, def bool) field {
	isSet := func(v *bool) bool { return v != nil }
	if v, path, ok := innermost(layers, get, isSet); ok {
		return field{key: key, entries: []entry{{value: strconv.FormatBool(*v), source: path}}}
	}

	return field{key: key, entries: []entry{{value: strconv.FormatBool(def)}}}
}

// listField resolves a list key: every layer's values, outermost first.
func listField(layers []rc.RCLayer, key string, get func(*rc.AgenticRC) []string) field {
	f := field{key: key, list: true}
	for _, layer := range layers {
		for _, v := range get(layer.RC) {
			f.entries = append(f.entries, entry{value: v, source: layer.Path})
		}
	}

	return f
}

// innermost returns the value from the innermost layer where isSet holds, and that layer's path.
func innermost[T any](layers []rc.RCLayer, get func(*rc.AgenticRC) T, isSet func(T) bool) (T, string, bool) {
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
func baseNames(agenticRC *rc.AgenticRC, versions map[string]string) []string {
	names := make([]string, len(agenticRC.Build.Bases))
	for i, name := range agenticRC.Build.Bases {
		if v, ok := versions[name]; ok {
			names[i] = name + "@" + v
		} else {
			names[i] = name
		}
	}
	return names
}

// customInstallNames returns the names of rc's custom installs.
func customInstallNames(agenticRC *rc.AgenticRC) []string {
	names := make([]string, len(agenticRC.Build.CustomInstalls))
	for i, ci := range agenticRC.Build.CustomInstalls {
		names[i] = ci.Name
	}
	return names
}

// envKeys returns the keys of rc's env entries, leaving out values that may hold secrets.
func envKeys(agenticRC *rc.AgenticRC) []string {
	keys := make([]string, len(agenticRC.Run.Env))
	for i, env := range agenticRC.Run.Env {
		keys[i], _, _ = strings.Cut(env, "=")
	}
	return keys
}

// toolArgs returns each tool's args in rc as "tool: args", sorted by tool.
func toolArgs(agenticRC *rc.AgenticRC) []string {
	tools := slices.Sorted(maps.Keys(agenticRC.Run.Args))
	entries := make([]string, len(tools))
	for i, tool := range tools {
		entries[i] = tool + ": " + strings.Join(agenticRC.Run.Args[tool], " ")
	}
	return entries
}

// allowedPaths returns each of rc's path rules as its host followed by its paths.
func allowedPaths(agenticRC *rc.AgenticRC) []string {
	rules := make([]string, len(agenticRC.Run.Proxy.AllowedPaths))
	for i, rule := range agenticRC.Run.Proxy.AllowedPaths {
		rules[i] = rule.Host + " " + strings.Join(rule.Paths, ", ")
	}
	return rules
}

// credentialNames returns each of rc's credentials as its preset, or its hosts when it has none.
func credentialNames(agenticRC *rc.AgenticRC) []string {
	names := make([]string, len(agenticRC.Run.Proxy.Credentials))
	for i, cred := range agenticRC.Run.Proxy.Credentials {
		names[i] = cred.Preset
		if names[i] == "" {
			names[i] = strings.Join(cred.Hosts, ", ")
		}
	}
	return names
}

// marketplaceNames returns each of rc's marketplaces as "name (url)", plus the tools it is limited to.
func marketplaceNames(agenticRC *rc.AgenticRC) []string {
	names := make([]string, len(agenticRC.Marketplaces))
	for i, m := range agenticRC.Marketplaces {
		names[i] = fmt.Sprintf("%s (%s)", m.Name, m.URL)
		if len(m.Tools) > 0 {
			names[i] += " tools: " + strings.Join(m.Tools, ", ")
		}
	}
	return names
}

// customInstructions summarizes rc's custom instructions by line count, since the text can be long.
func customInstructions(agenticRC *rc.AgenticRC) []string {
	custom := strings.TrimSpace(agenticRC.Run.Instructions.Custom)
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
func resolveEffectiveVersions(layers []rc.RCLayer) map[string]string {
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
