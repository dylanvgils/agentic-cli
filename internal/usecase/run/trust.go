package run

import (
	"fmt"
	"path/filepath"

	"github.com/dylanvgils/agentic-cli/internal/config"
)

// CheckContextTrust has the user trust dir before a docker_context set in dir's own config file picks the Docker daemon, since the agent can edit that file.
func CheckContextTrust(dir, toolHome string, layers []config.RCLayer, trustFlag bool, prompter Prompter) error {
	for _, layer := range layers {
		if layer.RC.DockerContext != "" && filepath.Dir(layer.Path) == dir {
			return checkTrust(dir, toolHome, trustFlag, prompter)
		}
	}
	return nil
}

// checkTrust errors unless dir is trusted, trusting it first when trustFlag is set or prompter approves.
func checkTrust(dir, toolHome string, trustFlag bool, prompter Prompter) error {
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
func checkCredentials(layers []config.RCLayer, toolHome string, prompter Prompter) error {
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
