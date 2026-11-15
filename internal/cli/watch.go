package cli

import (
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/watch"
	"github.com/spf13/cobra"
)

// watchCmdName names the hidden command a run reruns agentic as, to tear the run down once it exits.
const watchCmdName = "__watch"

var watchCmd = &cobra.Command{
	Use:               watchCmdName,
	Short:             "Tear a run down once its agentic process exits",
	Hidden:            true,
	Args:              cobra.NoArgs,
	ValidArgsFunction: cobra.NoFileCompletions,
	PersistentPreRunE: skipRootPreRun,
	RunE:              runWatch,
}

func init() {
	rootCmd.AddCommand(watchCmd)

	watchCmd.Flags().String(watch.FlagRun, "", "ID of the run to tear down")
	watchCmd.Flags().String(watch.FlagRuntime, "", "runtime of the run")
	watchCmd.Flags().Int(watch.FlagPID, 0, "agentic process to wait for")
	_ = watchCmd.MarkFlagRequired(watch.FlagRun)
	_ = watchCmd.MarkFlagRequired(watch.FlagPID)
}

func runWatch(cmd *cobra.Command, _ []string) error {
	target, err := watchTarget(cmd)
	if err != nil {
		return err
	}

	runs := newRunServiceFor(docker.New(target.DockerContext))
	return watch.New(runs).Watch(target)
}

// watchTarget reads the watchdog's target from cmd's flags, with the root's --home and --docker-context.
func watchTarget(cmd *cobra.Command) (watch.Target, error) {
	runID, _ := cmd.Flags().GetString(watch.FlagRun)
	runtime, _ := cmd.Flags().GetString(watch.FlagRuntime)
	dockerContext, _ := cmd.Flags().GetString(dockerContextFlagName)
	pid, _ := cmd.Flags().GetInt(watch.FlagPID)

	id, err := sandbox.ParseRunID(runID)
	if err != nil {
		return watch.Target{}, err
	}
	return watch.Target{
		RunID:         id,
		Runtime:       sandbox.Runtime(runtime),
		ToolHome:      toolHome,
		DockerContext: dockerContext,
		PID:           pid,
	}, nil
}

// skipRootPreRun skips the root's pre-run for a detached command: no TTY for the context trust prompt, no upgrade offer.
func skipRootPreRun(_ *cobra.Command, _ []string) error {
	return nil
}
