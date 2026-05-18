package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/platform"
)

// BuildOptions controls how a tool image is built.
type BuildOptions struct {
	BaseOverride string            // overrides the tool's default base extras (comma-separated)
	NoCache      bool              // disable layer cache for all steps
	NoCacheTool  bool              // disable layer cache for the tool step only (used by update)
	NodeVersion  string            // override Node.js version
	Versions     map[string]string // extra name → version override, e.g. {"java": "21"}
}

// BuildTool runs the four-step multi-stage build pipeline for a tool.
// versionCmd is run inside the built image to detect the installed version (may be empty).
func BuildTool(toolDir, image, versionCmd, repoRoot string, opts BuildOptions) error {
	nodeVer, err := buildNodeBase(repoRoot, opts)
	if err != nil {
		return fmt.Errorf("node base: %w", err)
	}

	prevImage, baseLabel, err := buildExtraLayers(repoRoot, parseExtras(opts.BaseOverride), nodeVer, opts)
	if err != nil {
		return err
	}

	if err := buildToolImage(toolDir, image, prevImage, baseLabel, opts); err != nil {
		return fmt.Errorf("tool image: %w", err)
	}

	if versionCmd != "" {
		stampToolVersion(image, versionCmd)
	}

	return nil
}

func buildNodeBase(repoRoot string, opts BuildOptions) (string, error) {
	nodeDir := filepath.Join(repoRoot, "tools", "base", "node")

	args := []string{"build"}
	if opts.NoCache {
		args = append(args, arg("no-cache"))
	}
	if opts.NodeVersion != "" {
		args = append(args, arg("build-arg", "NODE_VERSION="+opts.NodeVersion))
	}
	args = append(args, arg("tag", "agentic-base"), nodeDir)

	if err := runInteractive(args...); err != nil {
		return "", err
	}

	return detectBaseVersion("agentic-base", "agentic-version-node"), nil
}

func buildExtraLayers(repoRoot string, extras []string, nodeVer string, opts BuildOptions) (string, string, error) {
	prevImage := "agentic-base"
	extraVersions := make(map[string]string)

	for i, extra := range extras {
		extraDir := filepath.Join(repoRoot, "tools", "base", extra)
		if _, err := os.Stat(extraDir); os.IsNotExist(err) {
			return "", "", fmt.Errorf("unknown base %q (valid: %s)", extra, validExtras(repoRoot))
		}
		imageTag := "agentic-base-" + strings.Join(extras[:i+1], "-")

		args := []string{"build"}
		if opts.NoCache {
			args = append(args, arg("no-cache"))
		}
		args = append(args, arg("build-arg", "BASE_IMAGE="+prevImage))
		if ver := opts.Versions[extra]; ver != "" {
			args = append(args, arg("build-arg", strings.ToUpper(extra)+"_VERSION="+ver))
		}
		args = append(args, arg("tag", imageTag), extraDir)

		if err := runInteractive(args...); err != nil {
			return "", "", fmt.Errorf("%s layer: %w", extra, err)
		}

		extraVersions[extra] = detectBaseVersion(imageTag, "agentic-version-"+extra)
		prevImage = imageTag
	}

	return prevImage, buildBaseLabel(nodeVer, extras, extraVersions), nil
}

func parseExtras(base string) []string {
	var extras []string
	for extra := range strings.SplitSeq(base, ",") {
		if extra = strings.TrimSpace(extra); extra != "" {
			extras = append(extras, extra)
		}
	}
	return extras
}

func validExtras(repoRoot string) string {
	baseDir := filepath.Join(repoRoot, "tools", "base")
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return "(unavailable)"
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && e.Name() != "node" {
			names = append(names, e.Name())
		}
	}
	return strings.Join(names, ", ")
}

func buildToolImage(toolDir, image, baseImage, baseLabel string, opts BuildOptions) error {
	args := []string{"build"}
	if opts.NoCache || opts.NoCacheTool {
		args = append(args, arg("no-cache"))
	}

	args = append(args,
		arg("build-arg", "HOST_UID="+platform.GetUID()),
		arg("build-arg", "HOST_GID="+platform.GetGID()),
		arg("build-arg", "BASE_IMAGE="+baseImage),
		label(LabelBase, baseLabel),
		label(LabelBuilt, buildBuiltLabel()),
		arg("tag", image),
		toolDir,
	)

	return runInteractive(args...)
}
