package cli

import (
	"fmt"

	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/spf13/cobra"
)

var volumesCmd = &cobra.Command{
	Use:   "volumes",
	Short: "Manage named Docker volumes",
	Long:  "Manage named Docker volumes created by agentic.",
}

var volumesCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a named agentic-managed volume",
	Args:  cobra.ExactArgs(1),
	RunE:  runVolumeCreate,
}

var volumesListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all agentic-managed volumes",
	Args:    cobra.NoArgs,
	RunE:    runVolumeList,
}

var volumesRemoveCmd = &cobra.Command{
	Use:               "remove [name]",
	Aliases:           []string{"rm"},
	Short:             "Remove an agentic-managed volume, or all if no name given",
	Args:              cobra.MaximumNArgs(1),
	RunE:              runVolumeRemove,
	ValidArgsFunction: volumeNamesFunc,
}

func init() {
	rootCmd.AddCommand(volumesCmd)
	volumesCmd.AddCommand(volumesCreateCmd, volumesListCmd, volumesRemoveCmd)
}

func runVolumeCreate(_ *cobra.Command, args []string) error {
	name := args[0]
	if err := dockerClient.CreateVolume(name); err != nil {
		return err
	}
	logging.Infof("created volume %s", name)
	return nil
}

func runVolumeList(_ *cobra.Command, _ []string) error {
	out, err := dockerClient.ListVolumes()
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

func runVolumeRemove(_ *cobra.Command, args []string) error {
	if len(args) == 1 {
		if err := dockerClient.RemoveVolume(args[0]); err != nil {
			return err
		}
		logging.Infof("removed volume %s", args[0])
		return nil
	}

	return removeAllVolumes()
}

// removeAllVolumes lists every agentic-managed volume and removes them once the user confirms.
func removeAllVolumes() error {
	names, err := dockerClient.ListVolumeNames()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		logging.Infof("no agentic-managed volumes found")
		return nil
	}

	if !confirmVolumeRemoval(names) {
		return nil
	}

	logging.Infof("removing %d volume(s)", len(names))
	for _, n := range names {
		if err := dockerClient.RemoveVolume(n); err != nil {
			return err
		}
		logging.Stepf("deleted: %s", n)
	}
	return nil
}

// confirmVolumeRemoval lists names and asks the user to confirm removing them all.
func confirmVolumeRemoval(names []string) bool {
	// On stderr with the prompt, so the list and the question stay together
	logging.Infof("volumes to remove:")
	for _, n := range names {
		logging.Err.Detail(n)
	}

	logging.Promptf("remove all agentic-managed volumes? [y/N] ")
	return confirmed()
}
