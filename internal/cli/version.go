package cli

import (
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	RunE:  runVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func runVersion(cmd *cobra.Command, _ []string) error {
	_, err := fmt.Fprintln(cmd.OutOrStdout(), versionOutput())
	return err
}

func versionOutput() string {
	out := "agentic version " + buildinfo.Version
	if extras := versionExtras(); extras != "" {
		out += "\n\n" + extras
	}
	return out
}

func versionExtras() string {
	meta := []struct{ key, val string }{
		{"commit", buildinfo.Commit},
		{"built by", buildinfo.InstallMethod},
		{"built date", buildinfo.BuildDate},
	}

	var lines []string

	for _, m := range meta {
		if m.val != "" {
			lines = append(lines, fmt.Sprintf("%-12s: %s", m.key, m.val))
		}
	}

	return strings.Join(lines, "\n")
}
