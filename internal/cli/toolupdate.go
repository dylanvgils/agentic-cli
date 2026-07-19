package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// toolUpdateCheckInterval bounds how often `agentic run` re-checks a tool's
// upstream version.
const toolUpdateCheckInterval = 6 * time.Hour

var (
	latestToolVersion func(tool, installedLabel string) (string, bool, bool) = docker.LatestToolVersion
	toolUpdateStdin   io.Reader                                              = os.Stdin
	toolUpdateStderr  io.Writer                                              = os.Stderr
)

// checkToolUpdate checks upstream for a newer version of toolName at most once
// per toolUpdateCheckInterval and, on a TTY, offers to update immediately. A
// missed or errored check is silent; only an explicitly confirmed update that
// fails to build returns an error.
func checkToolUpdate(home string, rc *config.AgenticRC, toolName, image string) error {
	if rc.Run.CheckUpdates != nil && !*rc.Run.CheckUpdates {
		return nil
	}

	installed, latest, ok := fetchToolUpdateIfDue(home, toolName, image)
	if !ok {
		return nil
	}

	if !notifyToolUpdate(toolName, installed, latest) {
		return nil
	}

	return applyToolUpdate(toolName, image)
}

// fetchToolUpdateIfDue fetches the latest upstream version for toolName if its
// check interval has elapsed, saves the check timestamp, and returns
// (installed, latest, true) if a newer version is available. The timestamp is
// only saved on a successful fetch, so a failed fetch retries next time
// instead of backing off.
func fetchToolUpdateIfDue(home, toolName, image string) (installed, latest string, ok bool) {
	cfg, err := config.LoadConfig(home)
	if err != nil {
		return "", "", false
	}

	if !toolUpdateShouldCheck(cfg.LastToolVersionCheck, toolName) {
		return "", "", false
	}

	info, err := inspectImage(image)
	if err != nil || info == nil {
		return "", "", false
	}

	latestVersion, newer, fetchOK := latestToolVersion(toolName, info.Version)
	if !fetchOK {
		return "", "", false
	}

	if cfg.LastToolVersionCheck == nil {
		cfg.LastToolVersionCheck = make(map[string]time.Time)
	}
	cfg.LastToolVersionCheck[toolName] = time.Now()
	_ = cfg.Save(home)

	if !newer {
		return "", "", false
	}

	return docker.ParseVersion(info.Version), latestVersion, true
}

// toolUpdateShouldCheck reports whether tool is due for a check: never
// checked, or the interval has elapsed since it last was.
func toolUpdateShouldCheck(lastChecks map[string]time.Time, tool string) bool {
	last, ok := lastChecks[tool]
	if !ok {
		return true
	}
	return time.Since(last) >= toolUpdateCheckInterval
}

// notifyToolUpdate prints an update notice to stderr for toolName and, on a
// TTY, prompts to update immediately. It returns whether the user confirmed;
// on a non-TTY it just suggests `agentic update <tool>` and returns false.
func notifyToolUpdate(toolName, installed, latest string) bool {
	if !isTerminal() {
		fmt.Fprintf(toolUpdateStderr, "=> %s update available: %s (current: %s) - run: agentic update %s\n",
			toolName, latest, installed, toolName)
		return false
	}

	fmt.Fprintf(toolUpdateStderr, "=> %s update available: %s (current: %s)\n   update now? [y/N] ",
		toolName, latest, installed)

	scanner := bufio.NewScanner(toolUpdateStdin)
	return scanner.Scan() && strings.EqualFold(strings.TrimSpace(scanner.Text()), "y")
}

// applyToolUpdate rebuilds image for toolName, reusing the currently
// installed build options where possible. Unlike the CLI self-update prompt,
// it never exits the process - runTool must still start the tool container
// afterward, whether or not the update succeeded.
func applyToolUpdate(toolName, image string) error {
	opts := tools.BuildOptions{}
	if info, err := inspectImage(image); err == nil && info != nil {
		opts = recoverOpts(info, opts)
	}

	if err := updateOneTool(toolName, image, opts); err != nil {
		return fmt.Errorf("=> update failed: %v\n   run: agentic update %s", err, toolName)
	}

	return nil
}
