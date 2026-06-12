package cmd

import (
	"fmt"
	"os"
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
	addNamespaceFlag(updateCmd)
	addAllFlag(updateCmd)
}

type updateTarget struct {
	name  string
	image string
	opts  tools.BuildOptions
}

func runUpdate(cmd *cobra.Command, args []string) error {
	rc, err := config.FindAndLoadFromCwd()
	if err != nil {
		return err
	}

	namespace := resolveNamespace(cmd, rc)
	opts := buildOptsFromFlags(cmd, rc)
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	all, _ := cmd.Flags().GetBool("all")

	if dryRun {
		return dryRunUpdate(args, namespace, opts)
	}

	// Generate the cache-bust value once so multiple targets for the same tool
	// (e.g. --all updating it across namespaces) can still share cached layers.
	opts.CacheBust = docker.NewCacheBust()

	// For --all, RC config bases/apt must not prevent per-image label recovery:
	// images in other namespaces may have been built with different configs.
	// Only explicit CLI flags and env vars should override all images.
	if all {
		if !cmd.Flags().Changed("base") && os.Getenv(config.EnvBaseOverride) == "" {
			opts.BaseOverride = ""
		}
		if !cmd.Flags().Changed("apt") && os.Getenv(config.EnvAptPackages) == "" {
			opts.AptPackages = nil
		}
	}

	targets, err := resolveUpdateTargets(args, namespace, opts, all)
	if err != nil {
		return err
	}

	if len(targets) == 0 {
		if all {
			fmt.Println("No agentic images found. Run 'agentic build' first.")
		} else if len(args) == 0 {
			fmt.Println("No tools are built. Run 'agentic build' first.")
		}
		return nil
	}

	for _, t := range targets {
		if err := updateOneTool(t.name, t.image, t.opts); err != nil {
			return err
		}
	}

	return pruneAndReport()
}

func resolveUpdateTargets(args []string, namespace string, opts tools.BuildOptions, all bool) ([]updateTarget, error) {
	if all {
		return resolveAllUpdateTargets(args, opts)
	}
	return resolveScopedUpdateTargets(args, namespace, opts)
}

func resolveAllUpdateTargets(args []string, opts tools.BuildOptions) ([]updateTarget, error) {
	var filters []docker.ImageFilter
	if len(args) > 0 {
		filters = append(filters, docker.ToolFilter(args[0]))
	}

	images, err := listAllImages(filters...)
	if err != nil {
		return nil, err
	}

	var targets []updateTarget
	for _, info := range images {
		if info.Tool == "" {
			continue
		}
		targets = append(targets, updateTarget{name: info.Tool, image: info.Image, opts: recoverOpts(info, opts)})
	}
	return targets, nil
}

func resolveScopedUpdateTargets(args []string, namespace string, opts tools.BuildOptions) ([]updateTarget, error) {
	skipUnbuilt := len(args) == 0
	var targets []updateTarget

	for _, name := range toolNames(args) {
		image, err := tools.ImageName(name, namespace)
		if err != nil {
			return nil, err
		}

		info, err := inspectImage(image)
		if err != nil {
			return nil, err
		}

		if skipUnbuilt && info == nil {
			output.Stepf("%s (skipped - not built)", image)
			continue
		}

		toolOpts := opts
		if info != nil {
			toolOpts = recoverOpts(info, opts)
		}

		targets = append(targets, updateTarget{name: name, image: image, opts: toolOpts})
	}

	return targets, nil
}

func dryRunUpdate(args []string, namespace string, opts tools.BuildOptions) error {
	if len(args) == 0 {
		return fmt.Errorf("--dry-run requires a tool argument")
	}

	image, err := tools.ImageName(args[0], namespace)
	if err == nil {
		if info, iErr := inspectImage(image); iErr == nil && info != nil {
			opts = recoverOpts(info, opts)
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

func recoverOpts(info *docker.ImageInfo, opts tools.BuildOptions) tools.BuildOptions {
	if opts.BaseOverride == "" {
		opts.BaseOverride = docker.RecoverExtras(info.Base)
	}
	if info.Apt != "" {
		recoveredPkgs := docker.RecoverApt(info.Apt)
		opts.AptPackages = tools.MergePackages(recoveredPkgs, opts.AptPackages)
	}
	return opts
}

func updateOneTool(name, image string, opts tools.BuildOptions) error {
	output.Step(image)
	if opts.BaseOverride != "" {
		output.Detailf("base: %s", strings.ReplaceAll(opts.BaseOverride, ",", ", "))
	}
	if len(opts.AptPackages) > 0 {
		output.Detailf("apt: %s", strings.Join(opts.AptPackages, ", "))
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
	if after == "" {
		return
	}

	if before == "" {
		output.Detailf("version: %s", after)
		return
	}

	if before != after {
		output.Detailf("version: %s -> %s", before, after)
	} else {
		output.Detailf("version: %s (up to date)", after)
	}
}
