package run

import (
	"fmt"

	"github.com/dylanvgils/agentic-cli/internal/config"
)

// checkTrust errors unless dir is trusted, trusting it first when trustFlag is set or prompter approves.
func (s *Service) checkTrust(dir, toolHome string, trustFlag bool, prompter Prompter) error {
	cfg, err := config.LoadConfig(toolHome)
	if err != nil {
		return fmt.Errorf("load trust config: %w", err)
	}

	if cfg.IsTrusted(dir) {
		return nil
	}

	if !trustFlag {
		if err := prompter.TrustDir(dir); err != nil {
			return err
		}
	}

	return cfg.Trust(dir, toolHome)
}

// checkCredentials has prompter approve each layer's proxy credentials when they first appear and whenever they change, since an agent can edit a config file in the workspace.
func (s *Service) checkCredentials(layers []config.RCLayer, toolHome string, prompter Prompter) error {
	cfg, err := config.LoadConfig(toolHome)
	if err != nil {
		return fmt.Errorf("load trust config: %w", err)
	}

	for _, layer := range cfg.PendingCredentials(layers) {
		if err := prompter.ApproveCredentials(layer); err != nil {
			return err
		}

		hash := config.CredentialsHash(layer.RC.Run.Proxy.Credentials)
		if err := cfg.ApproveCredentials(layer.Path, hash, toolHome); err != nil {
			return fmt.Errorf("save credential approval: %w", err)
		}
	}

	return nil
}
