package cli

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Docker and microsandbox status and how many agentic containers and VMs are running",
	RunE:  runStatus,
}

// statusDetail is the status lines; a count that couldn't be read is left empty.
type statusDetail struct {
	docker        string `field:"Docker"`
	dockerContext string `field:"Docker context"`
	microsandbox  string `field:"Microsandbox"`
	containers    string `field:"Containers running"`
	vms           string `field:"VMs running"`
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(_ *cobra.Command, _ []string) error {
	// A failed lookup shouldn't hide the rest of the status
	ctx, _ := dockerClient.CurrentContext()

	daemonErr := dockerClient.CheckDaemon()
	docker := "running"
	if daemonErr != nil {
		docker = "not running"
	}
	detail := statusDetail{
		docker:        docker,
		dockerContext: orDash(ctx),
		microsandbox:  msbStatus(),
	}

	if daemonErr == nil {
		containers, err := dockerClient.ListRunningContainers()
		if err != nil {
			return err
		}
		detail.containers = strconv.Itoa(len(containers))
	}

	if msbClient.Available() == nil {
		if sandboxes, err := msbClient.ListRunning(); err != nil {
			logging.Warnf("%v", err)
		} else {
			detail.vms = strconv.Itoa(len(sandboxes))
		}
	}

	return writeFields(logging.Log.Writer(), "", detail)
}

// msbStatus reports whether the optional msb binary is installed.
func msbStatus() string {
	if msbClient.Available() != nil {
		return "not installed"
	}
	return "installed"
}
