package cli

import (
	"context"
	"os"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/spf13/cobra"
)

// projectKey keys the command's loaded project on its context.
type projectKey struct{}

// loadedProject is a project load's result, error included, so each reader decides whether a broken rc is fatal.
type loadedProject struct {
	project config.Project
	err     error
}

// withProject loads the working dir's project and stores it on cmd's context; persistentPreRunE calls it first.
func withProject(cmd *cobra.Command) {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Always wrap again: Cobra keeps a command's context between Execute calls, so the newest load must win
	project, err := loadCurrentProject()
	cmd.SetContext(context.WithValue(ctx, projectKey{}, loadedProject{project: project, err: err}))
}

// currentProject returns the project persistentPreRunE loaded for the working dir, loading it now when RunE ran without pre-run or in another dir.
func currentProject(cmd *cobra.Command) (config.Project, error) {
	if ctx := cmd.Context(); ctx != nil {
		if loaded, ok := ctx.Value(projectKey{}).(loadedProject); ok {
			if cwd, err := os.Getwd(); err == nil && cwd == loaded.project.WorkDir {
				return loaded.project, loaded.err
			}
		}
	}

	return loadCurrentProject()
}

// projectNamespace returns the --namespace flag, else the current project's namespace.
func projectNamespace(cmd *cobra.Command) (string, error) {
	project, err := currentProject(cmd)
	if err != nil {
		return "", err
	}

	return resolveNamespace(cmd, project.RC), nil
}

// loadCurrentProject loads the project for the working dir.
func loadCurrentProject() (config.Project, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return config.Project{}, err
	}

	return config.LoadProject(cwd)
}
