package cli

import (
	"fmt"
	"slices"

	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/usecase/volumes"
	"github.com/spf13/cobra"
)

var volumesCmd = &cobra.Command{
	Use:   "volumes",
	Short: "Manage named volumes",
	Long:  "Manage named volumes created by agentic, in Docker and microsandbox.",
}

var volumesCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a named agentic-managed Docker volume",
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

	all, err := volumes.New(dockerClient, msbClient).List()
	if err != nil {
		return err
	}
	msbVolumes := slices.DeleteFunc(all, func(v volumes.Volume) bool { return v.Runtime != volumes.RuntimeMicrosandbox })
	if len(msbVolumes) > 0 {
		logging.Step(volumes.RuntimeMicrosandbox)
		for _, v := range msbVolumes {
			logging.Detail(v.Name)
		}
	}
	return nil
}

func runVolumeRemove(_ *cobra.Command, args []string) error {
	svc := volumes.New(dockerClient, msbClient)
	if len(args) == 1 {
		if err := svc.RemoveNamed(args[0]); err != nil {
			return err
		}
		logging.Infof("removed volume %s", args[0])
		return nil
	}

	return removeAllVolumes(svc)
}

// removeAllVolumes lists every agentic-managed volume and removes them once the user confirms.
func removeAllVolumes(svc *volumes.Service) error {
	all, err := svc.List()
	if err != nil {
		return err
	}
	if len(all) == 0 {
		logging.Infof("no agentic-managed volumes found")
		return nil
	}

	if !confirmVolumeRemoval(all) {
		return nil
	}

	logging.Infof("removing %d volume(s)", len(all))
	for _, v := range all {
		if err := svc.Remove(v); err != nil {
			return err
		}
		logging.Stepf("deleted: %s", volumeLabel(v))
	}
	return nil
}

// confirmVolumeRemoval lists all and asks the user to confirm removing them.
func confirmVolumeRemoval(all []volumes.Volume) bool {
	// On stderr with the prompt, so the list and the question stay together
	logging.Infof("volumes to remove:")
	for _, v := range all {
		logging.Err.Detail(volumeLabel(v))
	}

	logging.Promptf("remove all agentic-managed volumes? [y/N] ")
	return confirmed()
}

// volumeLabel names v, marking the volumes that aren't Docker's with their runtime.
func volumeLabel(v volumes.Volume) string {
	if v.Runtime == volumes.RuntimeDocker {
		return v.Name
	}
	return v.Name + " (" + v.Runtime + ")"
}
