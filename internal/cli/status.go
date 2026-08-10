package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Docker backend status and running agentic containers",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, _ []string) error {
	out := cmd.OutOrStdout()

	if err := checkDockerDaemon(); err != nil {
		_, err := fmt.Fprintln(out, "Docker: not running")
		return err
	}

	containers, err := listRunningContainers()
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

	if ctx := docker.Context(); ctx != "" {
		if _, err := fmt.Fprintf(w, "Docker context:\t%s\n", ctx); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(w, "Docker:\trunning\nContainers running:\t%d\n", len(containers)); err != nil {
		return err
	}

	if err := writeContainerStatus(w, containers); err != nil {
		return err
	}

	return w.Flush()
}

func writeContainerStatus(w *tabwriter.Writer, containers []*docker.ContainerInfo) error {
	if len(containers) == 0 {
		return nil
	}

	if _, err := fmt.Fprintln(w, "\nNAME\tNAMESPACE\tTOOL\tIMAGE\tSTATUS"); err != nil {
		return err
	}

	for _, c := range containers {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			orDash(c.Name), orDash(c.Namespace), orDash(c.Tool), orDash(c.Image), orDash(c.Status)); err != nil {
			return err
		}
	}

	return nil
}
