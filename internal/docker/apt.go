package docker

import (
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/output"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

const aptCheckScript = `apt-get update -qq >/dev/null 2>&1
for pkg in "$@"; do
  apt-cache show "$pkg" >/dev/null 2>&1 || echo "$pkg"
done`

// verifyAptPackages checks that all named packages exist in the bookworm apt index.
// It pulls debian:bookworm-slim (showing progress), then identifies any missing packages
// by name so the error is actionable. This runs before the Docker build so users get a
// clear error without waiting for layer construction.
func verifyAptPackages(packages []string) error {
	if len(packages) == 0 {
		return nil
	}

	output.Step("Verifying apt packages...")

	if err := runInteractive("pull", tools.DebianImage); err != nil {
		return fmt.Errorf("failed to pull verification image: %w", err)
	}

	missing, err := missingAptPackages(packages)
	if err != nil {
		return err
	}

	if len(missing) > 0 {
		return fmt.Errorf("apt packages not found: %s", strings.Join(missing, ", "))
	}

	return nil
}

// missingAptPackages returns the names of packages from the list that do not exist
// in the bookworm apt index. It assumes debian:bookworm-slim is already pulled.
func missingAptPackages(packages []string) ([]string, error) {
	args := append([]string{"run", arg("rm"), tools.DebianImage, "sh", "-c", aptCheckScript, "--"}, packages...)
	out, err := dockerRun(args...)
	if err != nil {
		return nil, fmt.Errorf("apt package verification failed: %w", err)
	}

	var missing []string
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			missing = append(missing, line)
		}
	}

	return missing, nil
}
