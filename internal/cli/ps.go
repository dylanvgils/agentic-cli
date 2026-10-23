package cli

import (
	"github.com/spf13/cobra"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/toolruns"
)

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List the running agentic containers and VMs in the active or specified namespace",
	Args:  cobra.NoArgs,
	RunE:  runPs,
}

// psRow is one ps table row.
type psRow struct {
	tool      string          `table:"TOOL"`
	namespace string          `table:"NAMESPACE"`
	runtime   sandbox.Runtime `table:"RUNTIME"`
	status    string          `table:"STATUS"`
	name      string          `table:"NAME"`
}

func init() {
	rootCmd.AddCommand(psCmd)

	addNamespaceFlag(psCmd)
	addAllFlag(psCmd)
}

func runPs(cmd *cobra.Command, _ []string) error {
	rc, err := config.FindAndLoadFromCwd()
	if err != nil {
		return err
	}

	namespace := resolveNamespace(cmd, rc)
	if all, _ := cmd.Flags().GetBool("all"); all {
		namespace = ""
	}

	svc := toolruns.New(dockerClient, msbClient)
	return writeRuns(namespace, svc.List(namespace))
}

// writeRuns prints runs, one row each; namespace is the one listed, or empty for every namespace.
func writeRuns(namespace string, runs []toolruns.Run) error {
	if len(runs) == 0 && namespace != "" {
		logging.Infof("no running containers or VMs in namespace %q", namespace)
		return nil
	}
	if len(runs) == 0 {
		logging.Infof("no running containers or VMs")
		return nil
	}

	rows := make([]psRow, len(runs))
	for i, r := range runs {
		rows[i] = psRow{
			tool:      r.Tool,
			namespace: r.Namespace,
			runtime:   r.Runtime,
			status:    r.Status,
			name:      r.Name,
		}
	}
	return writeTable(logging.Log.Writer(), rows)
}
