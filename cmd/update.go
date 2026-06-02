package cmd

import (
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/output"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update [tool]",
	Short: "Update tool image(s) to latest version",
	Long: "Update tool image(s) to latest version. Rebuilds the tool step without cache\n" +
		"so the installer fetches the latest version. Skips unbuilt tools when no tool\n" +
		"specified.\n\n" + extrasEnvDoc(),
	Example: `  agentic update
  agentic update claude
  agentic update claude --base java
  agentic update claude --base java,dotnet
  agentic update claude --no-cache`,
	Args:      cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
	ValidArgs: tools.Names(),
	RunE:      runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)

	updateCmd.Flags().Bool("no-cache", false, "also rebuild base layers (fully fresh build)")

	addBuildFlags(updateCmd)
	addPrefixFlag(updateCmd)
	addAllFlag(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	rc := config.FindAndLoadFromCwd()
	prefix := resolvePrefix(cmd, rc)
	opts := buildOptsFromFlags(cmd)
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	all, _ := cmd.Flags().GetBool("all")

	if dryRun {
		return dryRunUpdate(args, prefix, opts)
	}

	if all {
		return updateAllImages(opts)
	}

	if err := updateTools(args, prefix, opts); err != nil {
		return err
	}

	return pruneAndReport()
}

func dryRunUpdate(args []string, prefix string, opts tools.BuildOptions) error {
	if len(args) == 0 {
		return fmt.Errorf("--dry-run requires a tool argument")
	}

	if opts.BaseOverride == "" {
		image, err := tools.ImageName(args[0], prefix)
		if err == nil {
			if info, iErr := inspectImage(image); iErr == nil && info != nil {
				opts.BaseOverride = docker.RecoverExtras(info.Base)
			}
		}
	}

	output.Step(args[0])
	content, err := tools.GenerateDockerfile(args[0], opts)
	if err != nil {
		return err
	}

	_, err = fmt.Println(content)
	return err
}

func updateAllImages(opts tools.BuildOptions) error {
	images, err := listAllImages()
	if err != nil {
		return err
	}

	if len(images) == 0 {
		fmt.Println("No agentic images found. Run 'agentic build' first.")
		return nil
	}

	updated := 0
	for _, info := range images {
		if info.Tool == "" {
			continue
		}
		output.Stepf("%s/%s", info.Prefix, info.Tool)
		o := opts
		if o.BaseOverride == "" {
			o.BaseOverride = docker.RecoverExtras(info.Base)
		}
		if o.AptPackages == nil && info.Apt != "" {
			o.AptPackages = splitCommaSep(info.Apt)
		}
		before := docker.ParseVersion(info.Version)
		if err := updateTool(info.Tool, info.Image, o); err != nil {
			return err
		}
		after := imageVersion(info.Image)
		reportVersionChange(before, after)
		updated++
	}

	if updated == 0 {
		fmt.Println("No agentic images found. Run 'agentic build' first.")
	}

	return pruneAndReport()
}

func updateTools(args []string, prefix string, opts tools.BuildOptions) error {
	skipUnbuilt := len(args) == 0
	updated := 0

	for _, name := range toolNames(args) {
		if skipUnbuilt {
			image, err := tools.ImageName(name, prefix)
			if err != nil {
				return err
			}
			info, err := inspectImage(image)
			if err != nil {
				return err
			}
			if info == nil {
				output.Stepf("%s (skipped - not built)", name)
				continue
			}
		}

		if err := updateOneTool(name, prefix, opts); err != nil {
			return err
		}
		updated++
	}

	if skipUnbuilt && updated == 0 {
		fmt.Println("No tools are built. Run 'agentic build' first.")
	}

	return nil
}

func updateOneTool(name, prefix string, opts tools.BuildOptions) error {
	output.Step(name)

	image, err := tools.ImageName(name, prefix)
	if err != nil {
		return err
	}

	before := imageVersion(image)

	if err := updateTool(name, image, opts); err != nil {
		return err
	}

	after := imageVersion(image)
	reportVersionChange(before, after)
	return nil
}

func imageVersion(image string) string {
	info, err := inspectImage(image)
	if err != nil || info == nil {
		return ""
	}
	return docker.ParseVersion(info.Version)
}

func reportVersionChange(before, after string) {
	if before != "" && before != after {
		output.Stepf("version: %s -> %s", before, after)
	} else if after != "" {
		output.Stepf("version: %s (up to date)", after)
	}
}

func splitCommaSep(s string) []string {
	var result []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			result = append(result, p)
		}
	}
	return result
}
