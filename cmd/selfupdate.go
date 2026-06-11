package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/selfupdate"
	"github.com/spf13/cobra"
)

var (
	fetchLatestVersion func() (string, error) = selfupdate.LatestVersion
	performUpdate      func(string) error      = selfupdate.Update
	selfupdateStdin    io.Reader               = os.Stdin
	selfupdateStderr   io.Writer               = os.Stderr
)

var selfupdateCmd = &cobra.Command{
	Use:   "selfupdate",
	Short: "Update agentic to the latest release",
	Args:  cobra.NoArgs,
	RunE:  runSelfUpdate,
}

func init() {
	rootCmd.AddCommand(selfupdateCmd)
}

func runSelfUpdate(cmd *cobra.Command, _ []string) error {
	fmt.Fprintln(cmd.OutOrStdout(), "=> checking for updates...")

	latest, err := fetchLatestVersion()
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}

	if !selfupdate.IsNewer(version, latest) {
		fmt.Fprintf(cmd.OutOrStdout(), "=> already up to date (%s)\n", version)
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "=> updating %s -> %s...\n", version, latest)

	if err := performUpdate(latest); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "=> updated to %s\n", latest)
	return nil
}

// maybeNotifyUpdate checks GitHub for a newer release at most once per CheckInterval and
// notifies the user on stderr. On a TTY it prompts to update immediately; otherwise it
// prints a one-liner suggesting `agentic selfupdate`.
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
		fmt.Fprintf(selfupdateStderr, "\n=> update available: %s (current: %s)\n   update now? [y/N] ", latest, version)

		scanner := bufio.NewScanner(selfupdateStdin)
		if scanner.Scan() && strings.EqualFold(strings.TrimSpace(scanner.Text()), "y") {
			fmt.Fprintln(selfupdateStderr, "=> updating...")

			if err := performUpdate(latest); err != nil {
				fmt.Fprintf(selfupdateStderr, "=> update failed: %v\n   run: agentic selfupdate\n", err)
			} else {
				fmt.Fprintf(selfupdateStderr, "=> updated to %s\n", latest)
			}
		}

		return
	}

	fmt.Fprintf(selfupdateStderr, "\n=> update available: %s (current: %s) - run: agentic selfupdate\n", latest, version)
}
