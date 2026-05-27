package cmd

import (
	"fmt"

	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
)

var aliasesCmd = &cobra.Command{
	Use:   "aliases",
	Short: "Print shell alias definitions for installed tools",
	Long:  "Print shell alias definitions for installed tools.\nSource the output to activate aliases: source <(agentic aliases)",
	RunE:  runAliases,
}

func init() {
	rootCmd.AddCommand(aliasesCmd)
}

func runAliases(_ *cobra.Command, _ []string) error {
	fmt.Println("# agentic tool aliases - source with: source <(agentic aliases)")

	for _, name := range tools.Names() {
		image, err := tools.ImageName(name)
		if err != nil {
			return err
		}

		info, err := inspectImage(image)
		if err != nil {
			return err
		}

		if info != nil {
			fmt.Printf("alias %s='agentic run %s'\n", name, name)
		}
	}

	return nil
}
