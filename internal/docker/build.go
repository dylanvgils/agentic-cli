package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/output"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// BuildTool generates a multi-stage Dockerfile for the named tool and builds it.
// The installed tool version is detected from the image via its embedded version script.
func BuildTool(tool, image string, opts tools.BuildOptions) error {
	if opts.VerifyApt {
		if err := verifyAptPackages(opts.AptPackages, opts.Registry); err != nil {
			return err
		}
		output.Step("Building image...")
	}

	content, err := tools.GenerateDockerfile(tool, opts)
	if err != nil {
		return err
	}

	if err := buildFromContent(content, image, tool, opts); err != nil {
		return fmt.Errorf("tool image: %w", err)
	}

	stampImageLabels(image, tool, opts.BaseOverride, opts.AptPackages, opts.Versions)

	return nil
}

// buildFromContent writes content to a temp Dockerfile and builds the image.
func buildFromContent(content, image, tool string, opts tools.BuildOptions) (retErr error) {
	tmpDir, err := writeTempDockerfile(content)
	if err != nil {
		return err
	}

	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil && retErr == nil {
			retErr = fmt.Errorf("remove temp dir: %w", err)
		}
	}()

	return buildImage(tmpDir, image, tool, opts)
}

// writeTempDockerfile creates a temp directory, writes content as a Dockerfile,
// and returns the directory path for use as the build context.
func writeTempDockerfile(content string) (tmpDir string, err error) {
	tmpDir, err = os.MkdirTemp("", "agentic-build-*")
	if err != nil {
		return "", fmt.Errorf("temp dir: %w", err)
	}

	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err = os.WriteFile(dockerfilePath, []byte(content), 0o600); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", fmt.Errorf("write Dockerfile: %w", err)
	}

	return tmpDir, nil
}

// buildImage assembles the docker build arguments and runs the build.
func buildImage(tmpDir, image, tool string, opts tools.BuildOptions) error {
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	namespace := strings.TrimSuffix(image, "-"+tool)

	args := []string{
		"build",
		label(LabelProject, LabelProjectVal),
		label(LabelBuilt, buildBuiltLabel()),
		label(LabelCLIVersion, CLIVersion),
		label(LabelNamespace, namespace),
		label(LabelTool, tool),
	}

	if opts.NoCache {
		args = append(args, arg("no-cache"))
	} else if opts.CacheBust != "" {
		args = append(args, arg("build-arg", "CACHEBUST="+opts.CacheBust))
	}

	args = append(
		args,
		arg("build-arg", "HOST_UID="+platform.GetUID()),
		arg("build-arg", "HOST_GID="+platform.GetGID()),
	)

	for _, name := range tools.BuildLayers(opts.BaseOverride) {
		if ver := opts.Versions[name]; ver != "" {
			args = append(args, arg("build-arg", strings.ToUpper(name)+"_VERSION="+ver))
		}
	}

	args = append(args,
		arg("tag", image),
		arg("file", dockerfilePath),
		tmpDir)

	return runInteractive(args...)
}
