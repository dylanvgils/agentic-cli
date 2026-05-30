package docker

import (
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// UpdateTool runs a build update for a tool.
// It recovers the base extras and apt packages from the existing image's labels when
// not already set (so updates preserve the original build configuration),
// then delegates to BuildTool with NoCacheTool enabled so only the tool stage skips cache.
func UpdateTool(tool, image string, opts tools.BuildOptions) error {
	hasUserApt := len(opts.AptPackages) > 0

	info, err := InspectImage(image)
	if err == nil && info != nil {
		if opts.BaseOverride == "" && info.Base != "" {
			opts.BaseOverride = RecoverExtras(info.Base)
		}

		if info.Apt != "" {
			recoveredPkgs := recoverAptPackages(info.Apt)
			opts.AptPackages = tools.MergePackages(recoveredPkgs, opts.AptPackages)
		}
	}

	opts.VerifyApt = hasUserApt
	opts.NoCacheTool = true
	return BuildTool(tool, image, opts)
}

// recoverAptPackages parses the agentic.apt label value into a slice of package names.
func recoverAptPackages(aptLabel string) []string {
	var pkgs []string
	for pkg := range strings.SplitSeq(aptLabel, ",") {
		if pkg = strings.TrimSpace(pkg); pkg != "" {
			pkgs = append(pkgs, pkg)
		}
	}
	return pkgs
}
