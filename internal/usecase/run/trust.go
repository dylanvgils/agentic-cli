package run

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/dylanvgils/agentic-cli/internal/config/agentichome"
	"github.com/dylanvgils/agentic-cli/internal/config/agenticrc"
)

// approvedConfig is a project's config once checkSettings approved its guarded settings; only checkSettings builds one,
// so reordering Resolve can't let a run use settings the user hasn't seen.
type approvedConfig struct {
	layers []agenticrc.Layer
	rc     *agenticrc.Config
}

// CheckContextTrust has the user approve changed settings before a docker_context from a config file picks the Docker daemon, and trust dir first when its own file sets it.
func CheckContextTrust(dir, toolHome string, layers []agenticrc.Layer, trustFlag bool, prompter Prompter) error {
	if !slices.ContainsFunc(layers, func(l agenticrc.Layer) bool { return l.Config.DockerContext != "" }) {
		return nil
	}

	if layer, ok := workspaceLayer(dir, layers); ok && layer.Config.DockerContext != "" {
		if err := checkTrust(dir, toolHome, trustFlag, prompter); err != nil {
			return err
		}
	}
	_, err := checkSettings(agenticrc.Project{Layers: layers}, toolHome, prompter)
	return err
}

// checkTrust errors unless dir is trusted, trusting it first when trustFlag is set or prompter approves.
func checkTrust(dir, toolHome string, trustFlag bool, prompter Prompter) error {
	state, err := agentichome.LoadState(toolHome)
	if err != nil {
		return fmt.Errorf("load trust state: %w", err)
	}

	if state.IsTrusted(dir) {
		return nil
	}

	if !trustFlag {
		if err := prompter.TrustDir(dir); err != nil {
			return err
		}
	}

	return state.Trust(dir, toolHome)
}

// checkSettings has prompter approve each layer's guarded settings whenever they change, since the agent can edit any config file in a dir it ran in.
// The returned approvedConfig is what a run reads its settings from, so none can be used before this approval.
func checkSettings(project agenticrc.Project, toolHome string, prompter Prompter) (approvedConfig, error) {
	state, err := agentichome.LoadState(toolHome)
	if err != nil {
		return approvedConfig{}, fmt.Errorf("load trust state: %w", err)
	}

	for _, layer := range project.Layers {
		changed := layer.ChangedSettings(state.ApprovedHashes(layer.Path))
		if len(changed) == 0 {
			continue
		}

		if err := prompter.ApproveSettings(layer, changed); err != nil {
			return approvedConfig{}, err
		}

		if err := state.ApproveSettings(layer.Path, layer.ApprovalHashes(), toolHome); err != nil {
			return approvedConfig{}, fmt.Errorf("save settings approval: %w", err)
		}
	}
	return approvedConfig{layers: project.Layers, rc: project.Config}, nil
}

// workspaceLayer returns dir's own config file from layers.
func workspaceLayer(dir string, layers []agenticrc.Layer) (agenticrc.Layer, bool) {
	for _, layer := range layers {
		if filepath.Dir(layer.Path) == dir {
			return layer, true
		}
	}
	return agenticrc.Layer{}, false
}
