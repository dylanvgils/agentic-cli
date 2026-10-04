package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/logging"
)

var trustStdin io.Reader = os.Stdin

func checkTrust(dir, toolHome string, trustFlag bool) error {
	config, err := config.LoadConfig(toolHome)
	if err != nil {
		return fmt.Errorf("load trust config: %w", err)
	}

	if config.IsTrusted(dir) {
		return nil
	}

	if trustFlag {
		return config.Trust(dir, toolHome)
	}

	if !isTerminal() {
		return fmt.Errorf("directory %q is not trusted; run interactively or pass --trust-dir to approve", dir)
	}

	logging.Promptf("trust directory %s? [y/N] ", dir)
	if confirmed() {
		return config.Trust(dir, toolHome)
	}

	return fmt.Errorf("directory not trusted")
}

// checkCredentials asks the user to approve each layer's proxy credentials when they first appear and whenever they change, since an agent can edit a config file in the workspace.
func checkCredentials(layers []config.RCLayer, toolHome string) error {
	cfg, err := config.LoadConfig(toolHome)
	if err != nil {
		return fmt.Errorf("load trust config: %w", err)
	}

	for _, layer := range cfg.PendingCredentials(layers) {
		creds := layer.RC.Run.Proxy.Credentials

		if !isTerminal() {
			return fmt.Errorf("proxy credentials in %s are new or changed; run interactively to approve them", layer.Path)
		}

		logging.Warnf("%s declares new or changed proxy credentials:", layer.Path)
		for _, cred := range creds {
			logging.Warnf("  %s", describeCredential(cred))
		}

		logging.Promptf("allow the proxy to read these secrets and send them to these hosts? [y/N] ")
		if !confirmed() {
			return fmt.Errorf("proxy credentials in %s not approved", layer.Path)
		}

		if err := cfg.ApproveCredentials(layer.Path, config.CredentialsHash(creds), toolHome); err != nil {
			return fmt.Errorf("save credential approval: %w", err)
		}
	}

	return nil
}

// describeCredential summarizes where an entry's secret is read from and where it is sent.
func describeCredential(cred config.RCCredential) string {
	target := "preset " + cred.Preset
	if cred.Preset == "" {
		target = fmt.Sprintf("header %s on %s", cred.Header, strings.Join(cred.Hosts, ", "))
	}

	desc := fmt.Sprintf("%s, secret %s", target, cred.Secret)
	if len(cred.Env) > 0 {
		desc += ", env " + strings.Join(cred.Env, ", ")
	}
	return desc
}

// confirmed reads one line from trustStdin and reports whether it is y or Y.
func confirmed() bool {
	// Read byte by byte so a later prompt still gets its own line
	var line []byte
	buf := make([]byte, 1)
	for {
		n, err := trustStdin.Read(buf)
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
