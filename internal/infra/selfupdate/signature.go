package selfupdate

import (
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
)

const (
	bundleSuffix = ".sigstore.json"
	certIdentity = "https://github.com/dylanvgils/agentic-cli/.github/workflows/nightly-release.yml@refs/heads/main"
	certIssuer   = "https://token.actions.githubusercontent.com"
)

// ErrSignatureInvalid means cosign rejected the release's checksums signature.
var ErrSignatureInvalid = errors.New("signature verification failed: release may be tampered with")

// checkSignature verifies the downloaded checksums at checksumsPath against their release bundle when cosign is set.
// A release without a bundle is unsigned, not an error; an invalid signature is.
func checkSignature(client *http.Client, cosign, checksumsURL, checksumsPath string) error {
	if cosign == "" {
		return nil
	}

	bundlePath := checksumsPath + bundleSuffix
	err := downloadFile(client, checksumsURL+bundleSuffix, bundlePath)
	if errors.Is(err, errNotFound) {
		logging.Detail("release is unsigned, verifying checksum only")
		return nil
	}
	if err != nil {
		return fmt.Errorf("downloading signature: %w", err)
	}

	logging.Detail("verifying signature...")
	return verifySignature(cosign, checksumsPath, bundlePath)
}

// cosignPath returns the cosign binary on PATH, or "" when it isn't installed.
func cosignPath() string {
	path, err := exec.LookPath("cosign")
	if err != nil {
		return ""
	}
	return path
}

// verifySignature checks the bundle signature of checksumsPath with cosign against the release workflow identity.
func verifySignature(cosign, checksumsPath, bundlePath string) error {
	cmd := exec.Command(cosign, // NOSONAR - the user's own cosign from PATH is intended
		"verify-blob",
		"--bundle", bundlePath,
		"--certificate-identity", certIdentity,
		"--certificate-oidc-issuer", certIssuer,
		checksumsPath,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrSignatureInvalid, strings.TrimSpace(string(out)))
	}

	return nil
}
