package cli

import (
	"fmt"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/selfupdate"
	"github.com/spf13/cobra"
)

var (
	upgradeForce       bool
	upgradeVersion     string
	fetchLatestVersion func() (string, error) = selfupdate.LatestVersion
	performUpdate      func(string) error     = selfupdate.Update
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
		logging.Step("checking for updates...")

		latest, err := fetchLatestVersion()
		if err != nil {
			return fmt.Errorf("checking for updates: %w", err)
		}

		target = latest
	}

	if !upgradeForce && upgradeVersion == "" && !selfupdate.IsNewer(buildinfo.Version, target) {
		logging.Detailf("already up to date (%s)", buildinfo.Version)
		return nil
	}

	logging.Stepf("updating %s -> %s...", buildinfo.Version, target)

	if err := performUpdate(target); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	logging.Detailf("updated to %s", target)
	return nil
}
