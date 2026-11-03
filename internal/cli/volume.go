package cli

import (
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/volumes"
	"github.com/spf13/cobra"
)

var volumeCmd = &cobra.Command{
	Use:   "volume",
	Short: "Manage named volumes",
	Long:  "Manage named volumes created by agentic, in Docker and microsandbox.",
}

var volumeCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a named agentic-managed Docker volume",
	Args:  cobra.ExactArgs(1),
	RunE:  runVolumeCreate,
}

var volumeListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all agentic-managed volumes",
	Args:    cobra.NoArgs,
	RunE:    runVolumeList,
}

var volumeRemoveCmd = &cobra.Command{
	Use:               "remove <name|--all>",
	Aliases:           []string{"rm"},
	Short:             "Remove an agentic-managed volume, or all of them with --all",
	Args:              cobra.MatchAll(cobra.MaximumNArgs(1), requireArgOrAll("volume name"), argOrAllExclusive),
	RunE:              runVolumeRemove,
	ValidArgsFunction: volumeNamesFunc,
}

var volumesCmd = &cobra.Command{
	Use:   "volumes",
	Short: "List all agentic-managed volumes (shortcut for volume list)",
	Args:  cobra.NoArgs,
	RunE:  runVolumeList,
}

// volumeRow is one line of the volume list table.
type volumeRow struct {
	name    string          `table:"NAME"`
	runtime sandbox.Runtime `table:"RUNTIME"`
}

func init() {
	rootCmd.AddCommand(volumeCmd, volumesCmd)
	volumeCmd.AddCommand(volumeCreateCmd, volumeListCmd, volumeRemoveCmd)

	addAllFlag(volumeRemoveCmd, "remove every agentic-managed volume")
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
	all, err := newVolumeService().List()
	if err != nil {
		return err
	}

	if len(all) == 0 {
		logging.Infof("no agentic-managed volumes found")
		return nil
	}

	rows := make([]volumeRow, len(all))
	for i, v := range all {
		rows[i] = volumeRow{
			name:    v.Name,
			runtime: v.Runtime,
		}
	}
	return writeTable(logging.Out, rows)
}

func runVolumeRemove(_ *cobra.Command, args []string) error {
	svc := newVolumeService()
	if len(args) == 1 {
		if err := svc.RemoveNamed(args[0]); err != nil {
			return err
		}
		logging.Infof("removed volume %s", args[0])
		return nil
	}

	return removeAllVolumes(svc)
}

// newVolumeService returns a volumes Service over Docker and, when installed, microsandbox.
func newVolumeService() *volumes.Service {
	var others []volumes.Runtime
	if msbClient.Available() == nil {
		others = append(others, volumes.Runtime{Name: sandbox.RuntimeMicrosandbox, Store: msbClient})
	}
	return volumes.New(dockerClient, others...)
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
		logging.Infof("nothing removed")
		return nil
	}

	logging.Infof("removing %d volume(s)", len(all))
	return svc.RemoveAll(all)
}

// confirmVolumeRemoval lists all and asks the user to confirm removing them.
func confirmVolumeRemoval(all []volumes.Volume) bool {
	logging.Infof("volumes to remove:")
	for _, v := range all {
		logging.Detail(v.Label())
	}

	logging.Promptf("remove all agentic-managed volumes? [y/N] ")
	return confirmed()
}
