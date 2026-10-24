package cli

import (
	"bytes"
	"fmt"
	"io"

	"github.com/dylanvgils/agentic-cli/internal/infra/buildinfo"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	RunE:  runVersion,
}

// buildDetail is the build metadata section of version output.
type buildDetail struct {
	commit    string `field:"commit"`
	builtBy   string `field:"built by"`
	builtDate string `field:"built date"`
}

// runtimeDetail is the runtime CLI versions section of version output; a runtime that isn't installed is left empty.
type runtimeDetail struct {
	docker       string `field:"docker"`
	microsandbox string `field:"microsandbox"`
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func runVersion(cmd *cobra.Command, _ []string) error {
	return writeVersion(cmd.OutOrStdout())
}

// writeVersion prints the agentic version, then each section with a value after a blank line.
func writeVersion(out io.Writer) error {
	if _, err := fmt.Fprintln(out, "agentic version "+buildinfo.Version); err != nil {
		return err
	}
	if err := writeVersionSection(out, versionExtras()); err != nil {
		return err
	}
	return writeVersionSection(out, runtimeVersions())
}

func versionExtras() buildDetail {
	return buildDetail{
		commit:    buildinfo.Commit,
		builtBy:   buildinfo.InstallMethod,
		builtDate: buildinfo.BuildDate,
	}
}

// runtimeVersions lists the versions of the installed runtime CLIs agentic drives.
func runtimeVersions() runtimeDetail {
	return runtimeDetail{
		docker:       versionOrEmpty(dockerClient.Version),
		microsandbox: versionOrEmpty(msbClient.Version),
	}
}

// writeVersionSection prints v's fields after a blank line, or nothing when every field is empty.
func writeVersionSection[T any](out io.Writer, v T) error {
	var buf bytes.Buffer
	if err := writeFields(&buf, "", v); err != nil {
		return err
	}
	if buf.Len() == 0 {
		return nil
	}

	_, err := fmt.Fprintf(out, "\n%s", buf.String())
	return err
}

// versionOrEmpty returns the version fn reports, or "" when it fails (e.g. the binary isn't installed).
func versionOrEmpty(fn func() (string, error)) string {
	v, err := fn()
	if err != nil {
		return ""
	}
	return v
}
