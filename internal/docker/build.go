package docker

import (
	"fmt"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"
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
// toolDir is the absolute path to the tool directory (contains Dockerfile).
// image is the target Docker image name (e.g. "agentic-claude").
// versionCmd is run inside the built image to detect the installed version (may be empty).
// repoRoot is the repository root (used to locate shared/base/ Dockerfiles).
func BuildTool(toolDir, image, versionCmd, repoRoot string, opts BuildOptions) error {
	nodeVer, err := buildNodeBase(repoRoot, opts)
	if err != nil {
		return fmt.Errorf("node base: %w", err)
	}

	base := opts.BaseOverride
	var extras []string
	for e := range strings.SplitSeq(base, ",") {
		if e = strings.TrimSpace(e); e != "" {
			extras = append(extras, e)
		}
	}

	prevImage, baseLabel, err := buildExtraLayers(repoRoot, extras, nodeVer, opts)
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
	nodeDir := filepath.Join(repoRoot, "shared", "base", "node")

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

func buildExtraLayers(repoRoot string, extras []string, nodeVer string, opts BuildOptions) (prevImage, baseLabel string, err error) {
	prevImage = "agentic-base"
	baseLabel = "node"
	if nodeVer != "" {
		baseLabel += "@" + nodeVer
	}

	tagSuffix := ""
	for _, extra := range extras {
		extraDir := filepath.Join(repoRoot, "shared", "base", extra)

		if tagSuffix != "" {
			tagSuffix += "-"
		}
		tagSuffix += extra
		imageTag := "agentic-base-" + tagSuffix

		args := []string{"build"}
		if opts.NoCache {
			args = append(args, arg("no-cache"))
		}
		args = append(args, arg("build-arg", "BASE_IMAGE="+prevImage))
		if ver := opts.Versions[extra]; ver != "" {
			args = append(args, arg("build-arg", strings.ToUpper(extra)+"_VERSION="+ver))
		}
		args = append(args, arg("tag", imageTag), extraDir)

		if err = runInteractive(args...); err != nil {
			return "", "", fmt.Errorf("%s layer: %w", extra, err)
		}

		extraVer := detectBaseVersion(imageTag, "agentic-version-"+extra)
		baseLabel += "," + extra
		if extraVer != "" {
			baseLabel += "@" + extraVer
		}
		prevImage = imageTag
	}

	return prevImage, baseLabel, nil
}

func buildToolImage(toolDir, image, baseImage, baseLabel string, opts BuildOptions) error {
	uid, gid := currentUserIDs()
	built := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	args := []string{"build"}
	if opts.NoCache || opts.NoCacheTool {
		args = append(args, arg("no-cache"))
	}
	args = append(args,
		arg("build-arg", "HOST_UID="+uid),
		arg("build-arg", "HOST_GID="+gid),
		arg("build-arg", "BASE_IMAGE="+baseImage),
		arg("label", "agentic.base="+baseLabel),
		arg("label", "agentic.built="+built),
		arg("tag", image),
		toolDir,
	)

	return runInteractive(args...)
}

// stampToolVersion detects the installed tool version and applies it as an image label.
// Runs best-effort: errors are silently ignored since a missing label is non-fatal.
func stampToolVersion(image, versionCmd string) {
	ver := detectToolVersion(image, versionCmd)
	if ver == "" {
		return
	}
	_, _ = dockerRunStdin(
		strings.NewReader("FROM "+image+"\n"),
		"build",
		arg("label", "agentic.tool.version="+ver),
		arg("tag", image),
		"-",
	)
}

func detectBaseVersion(image, script string) string {
	out, err := dockerRun("run", arg("rm"), image, script)
	if err != nil {
		return ""
	}
	return extractVersion(out)
}

func detectToolVersion(image, cmd string) string {
	out, err := dockerRun("run", arg("rm"), arg("entrypoint", ""), image, "sh", "-c", cmd)
	if err != nil {
		return ""
	}
	return extractVersion(out)
}

var versionRe = regexp.MustCompile(`[0-9]+(\.[0-9]+)*`)

func extractVersion(out string) string {
	line := strings.SplitN(out, "\n", 2)[0]
	line = strings.TrimRight(line, "\r")
	return versionRe.FindString(line)
}

// ParseVersion extracts the first semver-like token from a string.
// Used by cmd/update to normalize version labels for comparison.
func ParseVersion(s string) string {
	return versionRe.FindString(s)
}

func currentUserIDs() (uid, gid string) {
	u, err := user.Current()
	if err != nil {
		return "1000", "1000"
	}
	return u.Uid, u.Gid
}
