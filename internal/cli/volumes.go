package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/output"
	"github.com/spf13/cobra"
)

var volumesStdin io.Reader = os.Stdin

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
	if err := createVolume(name); err != nil {
		return err
	}
	output.Stepf("created: %s", name)
	return nil
}

func runVolumeList(_ *cobra.Command, _ []string) error {
	out, err := listVolumes()
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

func runVolumeRemove(_ *cobra.Command, args []string) error {
	if len(args) == 1 {
		if err := removeVolume(args[0]); err != nil {
			return err
		}
		output.Stepf("deleted: %s", args[0])
		return nil
	}

	names, err := listVolumeNames()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		fmt.Println("No agentic-managed volumes found.")
		return nil
	}

	fmt.Println("Volumes to remove:")
	for _, n := range names {
		fmt.Printf("  %s\n", n)
	}

	fmt.Print("Remove all agentic-managed volumes? [y/N] ")
	scanner := bufio.NewScanner(volumesStdin)
	scanner.Scan()
	answer := strings.TrimSpace(scanner.Text())
	if answer != "y" && answer != "Y" {
		return nil
	}

	for _, n := range names {
		if err := removeVolume(n); err != nil {
			return err
		}
		output.Stepf("deleted: %s", n)
	}
	return nil
}
