package cli

import (
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/run"
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

	watchCmd.Flags().Int(watch.FlagPID, 0, "agentic process to wait for")
	watchCmd.Flags().String(watch.FlagRuntime, "", "runtime of the run")
	watchCmd.Flags().String(watch.FlagRun, "", "ID of the run to tear down")
	_ = watchCmd.MarkFlagRequired(watch.FlagPID)
	_ = watchCmd.MarkFlagRequired(watch.FlagRun)
}

func runWatch(cmd *cobra.Command, _ []string) error {
	target, err := watchTarget(cmd)
	if err != nil {
		return err
	}

	// Teardown needs only the runtime clients
	runs := run.New(docker.New(target.DockerContext), msbClient, nil, nil, nil)
	return watch.New(runs).Watch(target)
}

// watchTarget reads the watchdog's target from cmd's flags, with the root's --home and --docker-context.
func watchTarget(cmd *cobra.Command) (watch.Target, error) {
	pid, _ := cmd.Flags().GetInt(watch.FlagPID)
	runtime, _ := cmd.Flags().GetString(watch.FlagRuntime)
	dockerContext, _ := cmd.Flags().GetString(dockerContextFlagName)
	runID, _ := cmd.Flags().GetString(watch.FlagRun)

	id, err := sandbox.ParseRunID(runID)
	if err != nil {
		return watch.Target{}, err
	}
	return watch.Target{
		PID:           pid,
		Runtime:       sandbox.Runtime(runtime),
		ToolHome:      toolHome,
		DockerContext: dockerContext,
		RunID:         id,
	}, nil
}

// skipRootPreRun skips the root's pre-run for a detached command: no TTY for the context trust prompt, no upgrade offer.
func skipRootPreRun(_ *cobra.Command, _ []string) error {
	return nil
}
