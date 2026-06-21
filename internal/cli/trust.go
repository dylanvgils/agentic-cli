package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
)

var trustStdin io.Reader = os.Stdin

func checkTrust(dir, toolHome string, trustFlag bool) error {
	config, err := config.LoadConfig(toolHome)
	if err != nil {
		return fmt.Errorf("load trust config: %w", err)
	}

	if config.IsTrusted(dir) {
		return nil
	}

	if trustFlag {
		return config.Trust(dir, toolHome)
	}

	if !isTerminal() {
		return fmt.Errorf("directory %q is not trusted; run interactively or pass --trust-dir to approve", dir)
	}

	fmt.Printf("Trust directory %s? [y/N] ", dir)
	scanner := bufio.NewScanner(trustStdin)
	scanner.Scan()
	if answer := strings.TrimSpace(scanner.Text()); answer == "y" || answer == "Y" {
		return config.Trust(dir, toolHome)
	}

	return fmt.Errorf("directory not trusted")
}
