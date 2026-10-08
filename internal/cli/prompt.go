package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/logging"
)

// ttyPrompter asks the user on the terminal; without one it refuses with a hint on how to approve instead.
type ttyPrompter struct{}

// TrustDir asks whether to trust dir.
func (ttyPrompter) TrustDir(dir string) error {
	if !isTerminal() {
		return fmt.Errorf("directory %q is not trusted; run interactively or pass --trust-dir to approve", dir)
	}

	logging.Promptf("trust directory %s? [y/N] ", dir)
	if !confirmed() {
		return fmt.Errorf("directory not trusted")
	}

	return nil
}

// ApproveCredentials shows a layer's credential entries and returns an error unless the user approves them.
func (ttyPrompter) ApproveCredentials(layer config.RCLayer) error {
	if !isTerminal() {
		return fmt.Errorf("proxy credentials in %s are new or changed; run interactively to approve them", layer.Path)
	}

	logging.Infof("%q declares new or changed proxy credentials:", layer.Path)
	for _, cred := range layer.RC.Run.Proxy.Credentials {
		logging.Infof("  %s", describeCredential(cred))
	}

	logging.Promptf("allow the proxy to read these secrets and send them to these hosts? [y/N] ")
	if !confirmed() {
		return fmt.Errorf("proxy credentials in %s not approved", layer.Path)
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

// describeCredential summarizes where an entry's secret is read from and where it is sent, quoting each value so escape sequences print as text.
func describeCredential(cred config.RCCredential) string {
	target := fmt.Sprintf("preset %q", cred.Preset)
	if cred.Preset == "" {
		target = fmt.Sprintf("header %q on %s", cred.Header, quoteAll(cred.Hosts))
	}

	desc := fmt.Sprintf("%s, secret %q", target, cred.Secret)
	if len(cred.Env) > 0 {
		desc += ", env " + quoteAll(cred.Env)
	}
	return desc
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
