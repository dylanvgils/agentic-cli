package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/output"
	"github.com/dylanvgils/agentic-cli/internal/selfupdate"
	"github.com/spf13/cobra"
)

var (
	upgradeForce       bool
	upgradeVersion     string
	fetchLatestVersion func() (string, error) = selfupdate.LatestVersion
	performUpdate      func(string) error     = selfupdate.Update
	upgradeStdin       io.Reader              = os.Stdin
	upgradeStderr      io.Writer              = os.Stderr
	osExit             func(int)              = os.Exit
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade agentic to the latest release",
	Args:  cobra.NoArgs,
	RunE:  runUpgrade,
}

func init() {
	rootCmd.AddCommand(upgradeCmd)

	upgradeCmd.Flags().BoolVar(&upgradeForce, "force", false, "reinstall even if already up to date")
	upgradeCmd.Flags().StringVar(&upgradeVersion, "version", "", "install a specific version (e.g. v1.2.0)")
}

func runUpgrade(_ *cobra.Command, _ []string) error {
	target := upgradeVersion

	if target == "" {
		output.Step("checking for updates...")

		latest, err := fetchLatestVersion()
		if err != nil {
			return fmt.Errorf("checking for updates: %w", err)
		}

		target = latest
	}

	if !upgradeForce && upgradeVersion == "" && !selfupdate.IsNewer(buildinfo.Version, target) {
		output.Detailf("already up to date (%s)", buildinfo.Version)
		return nil
	}

	output.Stepf("updating %s -> %s...", buildinfo.Version, target)

	if err := performUpdate(target); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	output.Detailf("updated to %s", target)
	return nil
}

// maybeNotifyUpdate checks GitHub for a newer release at most once per CheckInterval and
// notifies the user on stderr. On a TTY it prompts to update immediately; otherwise it
// prints a one-liner suggesting `agentic upgrade`.
func maybeNotifyUpdate(home string) {
	if buildinfo.IsDevBuild() {
		return
	}

	latest, ok := fetchUpdateIfDue(home)
	if !ok {
		return
	}

	notifyUpdate(latest)
}

// fetchUpdateIfDue checks whether the update interval has elapsed, fetches the latest
// version from GitHub, saves the check timestamp, and returns (latestVersion, true) if a
// newer version is available. Returns ("", false) in all other cases.
func fetchUpdateIfDue(home string) (string, bool) {
	config, err := config.LoadConfig(home)
	if err != nil {
		return "", false
	}

	if !selfupdate.ShouldCheck(config.LastUpdateCheck) {
		return "", false
	}

	latest, err := fetchLatestVersion()
	if err != nil {
		return "", false
	}

	now := time.Now()
	config.LastUpdateCheck = &now
	_ = config.Save(home)

	if !selfupdate.IsNewer(buildinfo.Version, latest) {
		return "", false
	}

	return latest, true
}

// notifyUpdate prints an update notice to stderr. On a TTY it prompts the user to update
// immediately; otherwise it prints a one-liner suggesting `agentic upgrade`.
func notifyUpdate(latest string) {
	if !isTerminal() {
		fmt.Fprintf(upgradeStderr, "=> update available: %s (current: %s) - run: agentic upgrade\n", latest, buildinfo.Version)
		return
	}

	fmt.Fprintf(upgradeStderr, "=> update available: %s (current: %s)\n   update now? [y/N] ", latest, buildinfo.Version)

	scanner := bufio.NewScanner(upgradeStdin)
	if scanner.Scan() && strings.EqualFold(strings.TrimSpace(scanner.Text()), "y") {
		fmt.Fprintln(upgradeStderr, "=> updating...")

		if err := performUpdate(latest); err != nil {
			fmt.Fprintf(upgradeStderr, "=> update failed: %v\n   run: agentic upgrade\n", err)
			osExit(1)
		} else {
			fmt.Fprintf(upgradeStderr, "=> updated to %s\n", latest)
			osExit(0)
		}
	}
}
