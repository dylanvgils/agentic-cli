package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show the effective agentic configuration",
	Long:  `Show the merged configuration from agentic.json and all .agenticrc.toml files.`,
	Args:  cobra.NoArgs,
	RunE:  showConfig,
}

// fieldPrinter prints project config fields one after another, keeping the first write error and skipping the rest.
type fieldPrinter struct {
	w      io.Writer
	layers []config.RCLayer
	err    error
}

// generalFields prints the top-level fields.
func (p *fieldPrinter) generalFields() {
	var (
		namespace     = func(rc *config.AgenticRC) string { return rc.Namespace }
		dockerContext = func(rc *config.AgenticRC) string { return rc.DockerContext }
	)

	p.scalar("namespace", namespace, config.DefaultNamespace)
	p.scalar("docker_context", dockerContext, "")
}

// buildFields prints the [build] fields.
func (p *fieldPrinter) buildFields() {
	aptPackages := func(rc *config.AgenticRC) []string { return rc.Build.AptPackages }

	p.bases()
	p.list("apt_packages", aptPackages)
	p.list("custom_installs", customInstallNames)
}

// runFields prints the [run] limits and mounts.
func (p *fieldPrinter) runFields() {
	var (
		pidsLimit      = func(rc *config.AgenticRC) string { return rc.Run.PidsLimit }
		cpus           = func(rc *config.AgenticRC) string { return rc.Run.CPUs }
		memory         = func(rc *config.AgenticRC) string { return rc.Run.Memory }
		extraMounts    = func(rc *config.AgenticRC) []string { return rc.Run.ExtraMounts }
		readOnlyMounts = func(rc *config.AgenticRC) []string { return rc.Run.ReadOnlyMounts }
		secrets        = func(rc *config.AgenticRC) []string { return rc.Run.Secrets }
	)

	p.scalar("pids_limit", pidsLimit, docker.DefaultPidsLimit)
	p.scalar("cpus", cpus, docker.DefaultCPUs)
	p.scalar("memory", memory, docker.DefaultMemory)
	p.list("extra_mounts", extraMounts)
	p.list("read_only_mounts", readOnlyMounts)
	p.list("secrets", secrets)
}

// proxyFields prints the [run.proxy] fields.
func (p *fieldPrinter) proxyFields() {
	var (
		enabled      = func(rc *config.AgenticRC) *bool { return rc.Run.Proxy.Enabled }
		mode         = func(rc *config.AgenticRC) string { return rc.Run.Proxy.Mode }
		allowedHosts = func(rc *config.AgenticRC) []string { return rc.Run.Proxy.AllowedHosts }
	)

	p.boolean("proxy.enabled", enabled, false)
	p.scalar("proxy.mode", mode, config.ModeEnforce)
	p.list("proxy.allowed_hosts", allowedHosts)
}

// dindFields prints the [run.dind] fields; unset limits inherit the tool's.
func (p *fieldPrinter) dindFields() {
	var (
		enabled       = func(rc *config.AgenticRC) *bool { return rc.Run.Dind.Enabled }
		pidsLimit     = func(rc *config.AgenticRC) string { return rc.Run.Dind.PidsLimit }
		cpus          = func(rc *config.AgenticRC) string { return rc.Run.Dind.CPUs }
		memory        = func(rc *config.AgenticRC) string { return rc.Run.Dind.Memory }
		toolPidsLimit = func(rc *config.AgenticRC) string { return rc.Run.PidsLimit }
		toolCPUs      = func(rc *config.AgenticRC) string { return rc.Run.CPUs }
		toolMemory    = func(rc *config.AgenticRC) string { return rc.Run.Memory }
	)

	p.boolean("dind.enabled", enabled, false)
	p.scalar("dind.pids_limit", pidsLimit, effectiveScalar(p.layers, toolPidsLimit, docker.DefaultPidsLimit))
	p.scalar("dind.cpus", cpus, effectiveScalar(p.layers, toolCPUs, docker.DefaultCPUs))
	p.scalar("dind.memory", memory, effectiveScalar(p.layers, toolMemory, docker.DefaultMemory))
}

// scalar prints a scalar field via printScalarField.
func (p *fieldPrinter) scalar(label string, get func(*config.AgenticRC) string, defaultVal string) {
	if p.err == nil {
		p.err = printScalarField(p.w, label, p.layers, get, defaultVal)
	}
}

// list prints a list field via printListField.
func (p *fieldPrinter) list(label string, get func(*config.AgenticRC) []string) {
	if p.err == nil {
		p.err = printListField(p.w, label, p.layers, get)
	}
}

// boolean prints a bool field via printBoolField.
func (p *fieldPrinter) boolean(label string, get func(*config.AgenticRC) *bool, defaultVal bool) {
	if p.err == nil {
		p.err = printBoolField(p.w, label, p.layers, get, defaultVal)
	}
}

// bases prints the bases field via printBasesField.
func (p *fieldPrinter) bases() {
	if p.err == nil {
		p.err = printBasesField(p.w, p.layers)
	}
}

func init() {
	rootCmd.AddCommand(configCmd)
}

func showConfig(cmd *cobra.Command, _ []string) error {
	cliConfig, err := config.LoadConfig(toolHome)
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
	if err := printGlobalConfig(w, toolHome, cliConfig); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	return printProjectConfig(w, layers)
}

func printGlobalConfig(w io.Writer, home string, cfg *config.CliConfig) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Global (%s)\n", filepath.Join(home, "agentic.json"))
	fmt.Fprintf(&b, "  registry: %s\n", orNotSet(cfg.Registry))
	fmt.Fprintf(&b, "  docker_context: %s\n", orNotSet(cfg.DockerContext))

	if len(cfg.TrustedDirs) == 0 {
		b.WriteString("  trusted_dirs: (none)\n")
	} else {
		b.WriteString("  trusted_dirs:\n")
		for _, dir := range cfg.TrustedDirs {
			fmt.Fprintf(&b, "    - %s\n", dir)
		}
	}

	_, err := io.WriteString(w, b.String())
	return err
}

func printProjectConfig(w io.Writer, layers []config.RCLayer) error {
	if len(layers) == 0 {
		if _, err := fmt.Fprintln(w, "Project (.agenticrc.toml)"); err != nil {
			return err
		}
		_, err := fmt.Fprintln(w, "  (no .agenticrc.toml files found)")
		return err
	}

	noun := "file"
	if len(layers) > 1 {
		noun = "files"
	}
	if _, err := fmt.Fprintf(w, "Project (.agenticrc.toml, %d %s)\n", len(layers), noun); err != nil {
		return err
	}

	p := &fieldPrinter{w: w, layers: layers}
	p.generalFields()
	p.buildFields()
	p.runFields()
	p.proxyFields()
	p.dindFields()

	return p.err
}

// customInstallNames returns the names of rc's custom installs.
func customInstallNames(rc *config.AgenticRC) []string {
	names := make([]string, len(rc.Build.CustomInstalls))
	for i, ci := range rc.Build.CustomInstalls {
		names[i] = ci.Name
	}
	return names
}

// orNotSet returns v, or "(not set)" when it is empty.
func orNotSet(v string) string {
	if v == "" {
		return "(not set)"
	}
	return v
}

// printScalarField prints a scalar config field: innermost RC value wins, else defaultVal tagged (default), else "(not set)".
func printScalarField(w io.Writer, label string, layers []config.RCLayer, get func(*config.AgenticRC) string, defaultVal string) error {
	for i := len(layers) - 1; i >= 0; i-- {
		if v := get(layers[i].RC); v != "" {
			_, err := fmt.Fprintf(w, "  %s: %s  [%s]\n", label, v, layers[i].Path)
			return err
		}
	}

	if defaultVal != "" {
		_, err := fmt.Fprintf(w, "  %s: %s  (default)\n", label, defaultVal)
		return err
	}

	_, err := fmt.Fprintf(w, "  %s: (not set)\n", label)
	return err
}

// effectiveScalar returns the innermost RC value for a scalar field, else defaultVal.
func effectiveScalar(layers []config.RCLayer, get func(*config.AgenticRC) string, defaultVal string) string {
	for i := len(layers) - 1; i >= 0; i-- {
		if v := get(layers[i].RC); v != "" {
			return v
		}
	}

	return defaultVal
}

// printBoolField prints a bool config field; the innermost layer with a non-nil value wins, else defaultVal is shown tagged (default).
func printBoolField(w io.Writer, label string, layers []config.RCLayer, get func(*config.AgenticRC) *bool, defaultVal bool) error {
	for i := len(layers) - 1; i >= 0; i-- {
		if v := get(layers[i].RC); v != nil {
			_, err := fmt.Fprintf(w, "  %s: %t  [%s]\n", label, *v, layers[i].Path)
			return err
		}
	}

	_, err := fmt.Fprintf(w, "  %s: %t  (default)\n", label, defaultVal)
	return err
}

// printListField prints a list config field, tagging each entry with its source layer, outermost-first.
func printListField(w io.Writer, label string, layers []config.RCLayer, get func(*config.AgenticRC) []string) error {
	type entry struct {
		value string
		path  string
	}

	var entries []entry
	for _, layer := range layers {
		for _, value := range get(layer.RC) {
			entries = append(entries, entry{value: value, path: layer.Path})
		}
	}

	if len(entries) == 0 {
		_, err := fmt.Fprintf(w, "  %s: (none)\n", label)
		return err
	}

	if _, err := fmt.Fprintf(w, "  %s:\n", label); err != nil {
		return err
	}

	for _, entry := range entries {
		if _, err := fmt.Fprintf(w, "    - %s  [%s]\n", entry.value, entry.path); err != nil {
			return err
		}
	}

	return nil
}

// printBasesField prints the bases list with versions inlined as basename@version.
func printBasesField(w io.Writer, layers []config.RCLayer) error {
	versions := resolveEffectiveVersions(layers)

	getBases := func(rc *config.AgenticRC) []string {
		result := make([]string, len(rc.Build.Bases))
		for i, name := range rc.Build.Bases {
			if v, ok := versions[name]; ok {
				result[i] = name + "@" + v
			} else {
				result[i] = name
			}
		}
		return result
	}

	return printListField(w, "bases", layers, getBases)
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
