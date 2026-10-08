package run

import (
	"fmt"

	"github.com/dylanvgils/agentic-cli/internal/config"
)

// CheckTrust errors unless dir is trusted, trusting it first when trustFlag is set or p approves.
func (s *Service) CheckTrust(dir, toolHome string, trustFlag bool, p Prompter) error {
	cfg, err := config.LoadConfig(toolHome)
	if err != nil {
		return fmt.Errorf("load trust config: %w", err)
	}

	if cfg.IsTrusted(dir) {
		return nil
	}

	if !trustFlag {
		if err := p.TrustDir(dir); err != nil {
			return err
		}
	}

	return cfg.Trust(dir, toolHome)
}

// CheckCredentials has p approve each layer's proxy credentials when they first appear and whenever they change, since an agent can edit a config file in the workspace.
func (s *Service) CheckCredentials(layers []config.RCLayer, toolHome string, p Prompter) error {
	cfg, err := config.LoadConfig(toolHome)
	if err != nil {
		return fmt.Errorf("load trust config: %w", err)
	}

	for _, layer := range cfg.PendingCredentials(layers) {
		if err := p.ApproveCredentials(layer); err != nil {
			return err
		}

		hash := config.CredentialsHash(layer.RC.Run.Proxy.Credentials)
		if err := cfg.ApproveCredentials(layer.Path, hash, toolHome); err != nil {
			return fmt.Errorf("save credential approval: %w", err)
		}
	}

	return nil
}
