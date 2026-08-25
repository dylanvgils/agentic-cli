package docker

import (
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

const aptCheckScript = `apt-get update -qq >/dev/null 2>&1
for pkg in "$@"; do
  apt-cache show "$pkg" >/dev/null 2>&1 || echo "$pkg"
done`

// verifyAptPackages checks that all named packages exist in the debian apt index before the
// Docker build starts, so a missing package fails fast with a clear error.
func verifyAptPackages(packages []string, registry string) error {
	if len(packages) == 0 {
		return nil
	}

	logging.Step("Verifying apt packages...")

	debianImage := tools.DebianImageFor(registry)
	if err := runInteractive("pull", debianImage); err != nil {
		return fmt.Errorf("failed to pull verification image: %w", err)
	}

	missing, err := missingAptPackages(packages, debianImage)
	if err != nil {
		return err
	}

	if len(missing) > 0 {
		return fmt.Errorf("apt packages not found: %s", strings.Join(missing, ", "))
	}

	return nil
}

// missingAptPackages returns names from packages absent from the debian apt index; image must already be pulled.
func missingAptPackages(packages []string, image string) ([]string, error) {
	args := append([]string{"run", arg("rm"), image, "sh", "-c", aptCheckScript, "--"}, packages...)
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
