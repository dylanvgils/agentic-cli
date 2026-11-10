package cli

import (
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/usecase/watch"
	"github.com/spf13/cobra"
)

// watchCmdName names the hidden command a run reruns agentic as, to tear the run down once it exits.
const watchCmdName = "__watch"

var watchCmd = &cobra.Command{
	Use:    watchCmdName,
	Short:  "Tear a run down once its agentic process exits",
	Hidden: true,
	// Its args are watch's own
	DisableFlagParsing: true,
	ValidArgsFunction:  cobra.NoFileCompletions,
	// Runs detached, so it skips the root's config, prompts and checks
	PersistentPreRunE: skipPreRun,
	RunE:              runWatch,
}

func init() {
	rootCmd.AddCommand(watchCmd)
}

func runWatch(_ *cobra.Command, args []string) error {
	target, err := watch.ParseTarget(args)
	if err != nil {
		return err
	}

	runs := newRunServiceFor(docker.New(target.DockerContext))
	return watch.New(runs).Watch(target)
}

// skipPreRun replaces the root's pre-run for commands that need none of it.
func skipPreRun(_ *cobra.Command, _ []string) error {
	return nil
}
