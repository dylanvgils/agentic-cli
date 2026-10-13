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

// versionField is one "key : value" line of version output.
type versionField struct{ key, val string }

func init() {
	rootCmd.AddCommand(versionCmd)
}

func runVersion(cmd *cobra.Command, _ []string) error {
	_, err := fmt.Fprintln(cmd.OutOrStdout(), versionOutput())
	return err
}

func versionOutput() string {
	out := "agentic version " + buildinfo.Version
	for _, section := range []string{versionExtras(), runtimeVersions()} {
		if section != "" {
			out += "\n\n" + section
		}
	}
	return out
}

func versionExtras() string {
	return formatFields([]versionField{
		{"commit", buildinfo.Commit},
		{"built by", buildinfo.InstallMethod},
		{"built date", buildinfo.BuildDate},
	})
}

// runtimeVersions lists the versions of the installed runtime CLIs agentic drives.
func runtimeVersions() string {
	return formatFields([]versionField{
		{"docker", versionOrEmpty(dockerClient.Version)},
		{"msb", versionOrEmpty(msbClient.Version)},
	})
}

// formatFields renders fields as aligned "key : value" lines, skipping empty values.
func formatFields(fields []versionField) string {
	var lines []string

	for _, f := range fields {
		if f.val != "" {
			lines = append(lines, fmt.Sprintf("%-12s: %s", f.key, f.val))
		}
	}

	return strings.Join(lines, "\n")
}

// versionOrEmpty returns the version fn reports, or "" when it fails (e.g. the binary isn't installed).
func versionOrEmpty(fn func() (string, error)) string {
	v, err := fn()
	if err != nil {
		return ""
	}
	return v
}
