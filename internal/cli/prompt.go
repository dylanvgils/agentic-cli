package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/logging"
)

// settingEffects says in a few words what each guarded setting controls, so the prompt shows what approving it allows.
var settingEffects = map[string]string{
	"root":                    "stops loading .agenticrc.toml files above this one",
	"namespace":               "which image set runs are started from",
	"docker_context":          "which Docker daemon runs the container",
	"marketplaces":            "plugin repos cloned and mounted into the container",
	"build.custom_installs":   "shell commands run at build time, unsandboxed",
	"run.extra_mounts":        "host paths mounted into the container",
	"run.read_only_mounts":    "host paths mounted read-only into the container",
	"run.secrets":             "host files mounted as secrets",
	"run.env":                 "host environment passed into the container",
	"run.proxy.enabled":       "whether egress is limited to the allowlist proxy",
	"run.proxy.mode":          "whether hosts off the allowlist are blocked or only logged",
	"run.proxy.allowed_hosts": "hosts the container may reach",
	config.CredentialsSetting: "host secrets the proxy reads and sends to these hosts",
	"run.dind.enabled":        "whether the container gets its own Docker daemon",
}

// ttyPrompter asks the user on the terminal; without one it refuses with a hint on how to approve instead.
type ttyPrompter struct{}

// TrustDir asks whether to trust dir.
func (ttyPrompter) TrustDir(dir string) error {
	if !isTerminal() {
		return fmt.Errorf("directory %s is not trusted; run interactively or pass --trust-dir to approve", describeDir(dir))
	}

	logging.Promptf("trust directory %s? [y/N] ", describeDir(dir))
	if !confirmed() {
		return fmt.Errorf("directory not trusted")
	}

	return nil
}

// ApproveSettings shows a layer's changed guarded settings and returns an error unless the user approves them.
func (ttyPrompter) ApproveSettings(layer config.RCLayer, changed []config.GuardedSetting) error {
	if !isTerminal() {
		return fmt.Errorf("settings in %s that reach outside the container changed (%s); run interactively to approve them", layer.Path, settingKeys(changed))
	}

	logging.Infof("%q has new or changed settings that reach outside the container.", layer.Path)
	logging.Infof("the agent can edit this file, so approve only changes you made or expect:")
	for _, setting := range changed {
		header, values := describeSetting(setting)
		logging.Err.Step(header)
		for _, value := range values {
			logging.Err.Detail(value)
		}
	}

	logging.Promptf("apply these settings? [y/N] ")
	if !confirmed() {
		return fmt.Errorf("settings in %s not approved", layer.Path)
	}

	return nil
}

// OfferToolUpdate announces an available tool update and, on a terminal, asks whether to apply it now; otherwise it just suggests `agentic update <tool>`.
func (ttyPrompter) OfferToolUpdate(tool, installed, latest string) bool {
	if !isTerminal() {
		logging.Infof("%s update available: %s (current: %s) - run: agentic update %s", tool, latest, installed, tool)
		return false
	}

	logging.Promptf("%s update available: %s (current: %s) - update now? [y/N] ", tool, latest, installed)
	return confirmed()
}

// OfferUpgrade announces a newer agentic release and, on a terminal, asks whether to upgrade now; otherwise it just suggests `agentic upgrade`.
func (ttyPrompter) OfferUpgrade(installed, latest string) bool {
	if !isTerminal() {
		logging.Err.Stepf("agentic update available: %s (current: %s) - run: agentic upgrade", latest, installed)
		return false
	}

	logging.Err.Stepf("agentic update available: %s (current: %s)", latest, installed)
	fmt.Fprint(logging.Err.Writer(), "   update now? [y/N] ")
	return confirmed()
}

// describeCredential summarizes where an entry's secret is sent, plus where it is read from and its env vars, quoting each value so escape sequences print as text.
func describeCredential(cred config.RCCredential) (string, []string) {
	target := fmt.Sprintf("preset %q", cred.Preset)
	if cred.Preset == "" {
		target = fmt.Sprintf("header %q on %s", cred.Header, quoteAll(cred.Hosts))
	}

	details := []string{fmt.Sprintf("secret %q", cred.Secret)}
	if len(cred.Env) > 0 {
		details = append(details, "env "+quoteAll(cred.Env))
	}
	return target, details
}

// confirmed reads one line from stdin and reports whether it is y or Y.
func confirmed() bool {
	// Read byte by byte so a later prompt still gets its own line
	var line []byte
	buf := make([]byte, 1)
	for {
		n, err := stdin.Read(buf)
		if n == 1 && buf[0] != '\n' {
			line = append(line, buf[0])
		}
		if (n == 1 && buf[0] == '\n') || err != nil {
			break
		}
	}

	answer := strings.TrimSpace(string(line))
	return answer == "y" || answer == "Y"
}

// quoteAll quotes each value and joins them with commas.
func quoteAll(values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = strconv.Quote(value)
	}
	return strings.Join(quoted, ", ")
}

// describeDir adds where dir really leads when it goes through a symlink, e.g. a link the agent planted.
func describeDir(dir string) string {
	if resolved, err := filepath.EvalSymlinks(dir); err == nil && resolved != dir {
		return dir + " (-> " + resolved + ")"
	}
	return dir
}

// describeSetting renders setting as a header with its key and effect, plus its value lines with one or more per list item; a cleared setting has no values.
func describeSetting(setting config.GuardedSetting) (string, []string) {
	header := setting.Key + " - " + settingEffects[setting.Key]
	if !setting.IsSet() {
		return header + " (removed)", nil
	}

	values := []any{setting.Value}
	if v := reflect.ValueOf(setting.Value); v.Kind() == reflect.Slice {
		values = make([]any, v.Len())
		for i := range values {
			values[i] = v.Index(i).Interface()
		}
	}

	var lines []string
	for _, value := range values {
		lines = append(lines, describeValue(value)...)
	}
	return header, lines
}

// describeValue renders a custom install as its name with one quoted command per line, a credential as where it is sent with its details below, and any other value as json.
func describeValue(value any) []string {
	switch value := value.(type) {
	case config.RCCustomInstall:
		lines := []string{strconv.Quote(value.Name) + ":"}
		for _, command := range value.Run {
			lines = append(lines, "  "+strconv.Quote(command))
		}
		return lines
	case config.RCCredential:
		target, details := describeCredential(value)
		lines := []string{target}
		for _, detail := range details {
			lines = append(lines, "  "+detail)
		}
		return lines
	}

	// Config values are plain data and always marshal
	data, _ := json.Marshal(value)
	return []string{string(data)}
}

// settingKeys returns the keys of settings, comma-separated.
func settingKeys(settings []config.GuardedSetting) string {
	keys := make([]string, 0, len(settings))
	for _, setting := range settings {
		keys = append(keys, setting.Key)
	}
	return strings.Join(keys, ", ")
}
