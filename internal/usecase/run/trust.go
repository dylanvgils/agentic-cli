package run

import (
	"fmt"
	"path/filepath"

	"github.com/dylanvgils/agentic-cli/internal/config"
)

// CheckContextTrust has the user trust dir and approve its own config file's guarded settings before a docker_context set there picks the Docker daemon, since the agent can edit that file.
func CheckContextTrust(dir, toolHome string, layers []config.RCLayer, trustFlag bool, prompter Prompter) error {
	layer, ok := workspaceLayer(dir, layers)
	if !ok || layer.RC.DockerContext == "" {
		return nil
	}

	if err := checkTrust(dir, toolHome, trustFlag, prompter); err != nil {
		return err
	}
	return checkSettings(dir, layers, toolHome, prompter)
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

// checkSettings has prompter approve the guarded settings of dir's own config file whenever they change, since the agent can edit that file but not the ones above dir.
func checkSettings(dir string, layers []config.RCLayer, toolHome string, prompter Prompter) error {
	layer, ok := workspaceLayer(dir, layers)
	if !ok {
		return nil
	}

	cfg, err := config.LoadConfig(toolHome)
	if err != nil {
		return fmt.Errorf("load trust config: %w", err)
	}

	changed := cfg.ChangedSettings(layer)
	if len(changed) == 0 {
		return nil
	}

	if err := prompter.ApproveSettings(layer, changed); err != nil {
		return err
	}

	if err := cfg.ApproveSettings(layer, toolHome); err != nil {
		return fmt.Errorf("save settings approval: %w", err)
	}
	return nil
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

// workspaceLayer returns dir's own config file from layers.
func workspaceLayer(dir string, layers []config.RCLayer) (config.RCLayer, bool) {
	for _, layer := range layers {
		if filepath.Dir(layer.Path) == dir {
			return layer, true
		}
	}
	return config.RCLayer{}, false
}
