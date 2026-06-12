package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/output"
	"github.com/dylanvgils/agentic-cli/internal/selfupdate"
	"github.com/spf13/cobra"
)

var (
	fetchLatestVersion func() (string, error) = selfupdate.LatestVersion
	performUpdate      func(string) error     = selfupdate.Update
	upgradeStdin       io.Reader              = os.Stdin
	upgradeStderr      io.Writer              = os.Stderr
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade agentic to the latest release",
	Args:  cobra.NoArgs,
	RunE:  runUpgrade,
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
}

func runUpgrade(_ *cobra.Command, _ []string) error {
	output.Step("checking for updates...")

	latest, err := fetchLatestVersion()
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}

	if !selfupdate.IsNewer(version, latest) {
		output.Stepf("already up to date (%s)", version)
		return nil
	}

	output.Stepf("updating %s -> %s...", version, latest)

	if err := performUpdate(latest); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	output.Stepf("updated to %s", latest)
	return nil
}

// maybeNotifyUpdate checks GitHub for a newer release at most once per CheckInterval and
// notifies the user on stderr. On a TTY it prompts to update immediately; otherwise it
// prints a one-liner suggesting `agentic upgrade`.
func maybeNotifyUpdate(home string) {
	if version == "dev" {
		return
	}

	cfg, err := config.LoadConfig(home)
	if err != nil {
		return
	}

	if !selfupdate.ShouldCheck(cfg.LastUpdateCheck) {
		return
	}

	latest, err := fetchLatestVersion()
	if err != nil {
		return
	}

	cfg.LastUpdateCheck = time.Now()
	_ = cfg.Save(home)

	if !selfupdate.IsNewer(version, latest) {
		return
	}

	if isTerminal() {
		fmt.Fprintf(upgradeStderr, "\n=> update available: %s (current: %s)\n   update now? [y/N] ", latest, version)

		scanner := bufio.NewScanner(upgradeStdin)
		if scanner.Scan() && strings.EqualFold(strings.TrimSpace(scanner.Text()), "y") {
			fmt.Fprintln(upgradeStderr, "=> updating...")

			if err := performUpdate(latest); err != nil {
				fmt.Fprintf(upgradeStderr, "=> update failed: %v\n   run: agentic upgrade\n", err)
			} else {
				fmt.Fprintf(upgradeStderr, "=> updated to %s\n", latest)
			}
		}

		return
	}

	fmt.Fprintf(upgradeStderr, "\n=> update available: %s (current: %s) - run: agentic upgrade\n", latest, version)
}
