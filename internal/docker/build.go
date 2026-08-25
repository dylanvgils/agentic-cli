package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/cleanup"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// BuildTool generates a multi-stage Dockerfile for the named tool, builds it, and stamps labels with the detected tool version.
func BuildTool(tool, image string, opts tools.BuildOptions) error {
	if opts.VerifyApt {
		if err := verifyAptPackages(opts.AptPackages, opts.Registry); err != nil {
			return err
		}
		logging.Step("Building image...")
	}

	content, err := tools.GenerateDockerfile(tool, opts)
	if err != nil {
		return err
	}

	if err := buildFromContent(content, image, tool, opts); err != nil {
		return fmt.Errorf("tool image: %w", err)
	}

	customInstallNames := make([]string, len(opts.CustomInstalls))
	for i, install := range opts.CustomInstalls {
		customInstallNames[i] = install.Name
	}
	stampImageLabels(image, tool, opts.BaseOverride, opts.AptPackages, opts.Versions, customInstallNames, opts.CacheBust)

	return nil
}

// BuildProxyImage generates the egress proxy Dockerfile and builds it: a released version
// installs the published module, a dev version compiles sourceDir (the agentic module root).
func BuildProxyImage(image, version, sourceDir string, opts tools.BuildOptions) (retErr error) {
	content := tools.GenerateProxyDockerfile(version, opts.Registry)

	tmpDir, err := writeTempDockerfile(content)
	if err != nil {
		return err
	}

	defer cleanup.Capture(&retErr, func() error {
		if err := os.RemoveAll(tmpDir); err != nil {
			return fmt.Errorf("remove temp dir: %w", err)
		}
		return nil
	})

	// Released builds need only the generated Dockerfile in context; dev builds
	// compile the local tree, so the source root becomes the build context.
	context := tmpDir
	if buildinfo.IsDev(version) {
		if sourceDir == "" {
			return fmt.Errorf("proxy image: dev build requires the agentic source tree - run \"agentic build\" from the repository, or build a published version")
		}
		context = sourceDir
	}

	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := buildProxyImage(dockerfilePath, image, context, opts); err != nil {
		return fmt.Errorf("proxy image: %w", err)
	}

	return nil
}

// buildProxyImage runs docker build for the proxy image.
func buildProxyImage(dockerfilePath, image, context string, opts tools.BuildOptions) error {
	return runInteractive(buildProxyImageArgs(dockerfilePath, image, context, opts)...)
}

// buildProxyImageArgs computes the docker build args for the proxy image: only the agentic
// labels (no tool/base build-args, no namespace label, since the proxy image is global).
func buildProxyImageArgs(dockerfilePath, image, context string, opts tools.BuildOptions) []string {
	args := []string{
		"build",
		label(LabelProject, LabelProjectVal),
		label(LabelBuilt, buildBuiltLabel()),
		label(LabelCLIVersion, buildinfo.Version),
		label(LabelTool, tools.ProxyImageSuffix),
	}

	if opts.NoCache {
		args = append(args, arg("no-cache"))
	}

	if opts.NoCache || opts.Pull {
		args = append(args, arg("pull"))
	}

	args = append(args,
		arg("tag", image),
		arg("file", dockerfilePath),
		context)

	return args
}

// buildFromContent writes content to a temp Dockerfile and builds the image.
func buildFromContent(content, image, tool string, opts tools.BuildOptions) (retErr error) {
	tmpDir, err := writeTempDockerfile(content)
	if err != nil {
		return err
	}

	defer cleanup.Capture(&retErr, func() error {
		if err := os.RemoveAll(tmpDir); err != nil {
			return fmt.Errorf("remove temp dir: %w", err)
		}
		return nil
	})

	return buildImage(tmpDir, image, tool, opts)
}

// writeTempDockerfile writes content as a Dockerfile in a fresh temp dir, returned as the build context.
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

// buildImage runs docker build for the tool image.
func buildImage(tmpDir, image, tool string, opts tools.BuildOptions) error {
	return runInteractive(buildImageArgs(tmpDir, image, tool, opts)...)
}

// buildImageArgs computes the docker build args for the tool image.
func buildImageArgs(tmpDir, image, tool string, opts tools.BuildOptions) []string {
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	namespace := strings.TrimSuffix(image, "-"+tool)

	args := []string{
		"build",
		label(LabelProject, LabelProjectVal),
		label(LabelBuilt, buildBuiltLabel()),
		label(LabelCLIVersion, buildinfo.Version),
		label(LabelNamespace, namespace),
		label(LabelTool, tool),
	}

	if opts.NoCache {
		args = append(args, arg("no-cache"))
	} else if opts.CacheBust != "" {
		args = append(args, arg("build-arg", "CACHEBUST="+opts.CacheBust))
	}

	if opts.NoCache || opts.Pull {
		args = append(args, arg("pull"), label(LabelPulled, buildPulledLabel()))
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

	return args
}
