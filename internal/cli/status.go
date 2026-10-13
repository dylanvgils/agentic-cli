package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Docker and microsandbox status and running agentic containers",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, _ []string) error {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

	// A failed lookup shouldn't hide the rest of the status
	ctx, _ := dockerClient.CurrentContext()
	msb := msbStatus()

	if err := dockerClient.CheckDaemon(); err != nil {
		if _, err := fmt.Fprintf(w, "Docker:\tnot running\nDocker context:\t%s\nMicrosandbox:\t%s\n", orDash(ctx), msb); err != nil {
			return err
		}
		return w.Flush()
	}

	containers, err := dockerClient.ListRunningContainers()
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "Docker:\trunning\nDocker context:\t%s\nMicrosandbox:\t%s\nContainers running:\t%d\n", orDash(ctx), msb, len(containers)); err != nil {
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

// msbStatus reports whether the optional msb binary is installed.
func msbStatus() string {
	if msbClient.Available() != nil {
		return "not installed"
	}
	return "installed"
}
