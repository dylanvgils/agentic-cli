package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Docker and microsandbox status and how many agentic containers and VMs are running",
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

	daemonErr := dockerClient.CheckDaemon()
	docker := "running"
	if daemonErr != nil {
		docker = "not running"
	}
	if _, err := fmt.Fprintf(w, "Docker:\t%s\nDocker context:\t%s\nMicrosandbox:\t%s\n", docker, orDash(ctx), msb); err != nil {
		return err
	}

	if daemonErr == nil {
		containers, err := dockerClient.ListRunningContainers()
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "Containers running:\t%d\n", len(containers)); err != nil {
			return err
		}
	}

	if msbClient.Available() == nil {
		if sandboxes, err := msbClient.ListRunning(); err != nil {
			logging.Warnf("%v", err)
		} else if _, err := fmt.Fprintf(w, "VMs running:\t%d\n", len(sandboxes)); err != nil {
			return err
		}
	}

	return w.Flush()
}

// msbStatus reports whether the optional msb binary is installed.
func msbStatus() string {
	if msbClient.Available() != nil {
		return "not installed"
	}
	return "installed"
}
