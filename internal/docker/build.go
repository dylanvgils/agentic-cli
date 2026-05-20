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

// BuildTool generates a multi-stage Dockerfile for the named tool and builds it.
// versionCmd is run inside the built image to detect the installed version (may be empty).
func BuildTool(tool, image, versionCmd string, opts BuildOptions) error {
	content, err := GenerateDockerfile(tool, opts)
	if err != nil {
		return err
	}

	if err := buildFromContent(content, image, opts); err != nil {
		return fmt.Errorf("tool image: %w", err)
	}

	stampImageLabels(image, versionCmd, parseExtras(opts.BaseOverride))

	return nil
}

// buildFromContent writes content to a temp Dockerfile and runs docker build.
func buildFromContent(content, image string, opts BuildOptions) error {
	tmpDir, err := os.MkdirTemp("", "agentic-build-*")
	if err != nil {
		return fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.WriteFile(filepath.Join(tmpDir, "Dockerfile"), []byte(content), 0o600); err != nil {
		return fmt.Errorf("write Dockerfile: %w", err)
	}

	args := []string{"build",
		"--file", filepath.Join(tmpDir, "Dockerfile"),
	}

	if opts.NoCache {
		args = append(args, arg("no-cache"))
	} else if opts.NoCacheTool {
		args = append(args, arg("no-cache-filter", "tool"))
	}

	args = append(args,
		arg("build-arg", "HOST_UID="+platform.GetUID()),
		arg("build-arg", "HOST_GID="+platform.GetGID()),
	)

	if opts.NodeVersion != "" {
		args = append(args, arg("build-arg", "NODE_VERSION="+opts.NodeVersion))
	}
	for _, extra := range parseExtras(opts.BaseOverride) {
		if ver := opts.Versions[extra]; ver != "" {
			args = append(args, arg("build-arg", strings.ToUpper(extra)+"_VERSION="+ver))
		}
	}

	args = append(args,
		label(LabelBuilt, buildBuiltLabel()),
		arg("tag", image),
		tmpDir,
	)

	return runInteractive(args...)
}
