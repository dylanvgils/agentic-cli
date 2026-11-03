package selfupdate

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	bundleSuffix  = ".sigstore.json"
	certIdentity  = "https://github.com/dylanvgils/agentic-cli/.github/workflows/nightly-release.yml@refs/heads/main"
	certIssuer    = "https://token.actions.githubusercontent.com"
	cosignTimeout = time.Minute

	// cosignWaitDelay bounds the wait for cosign's output pipes after a timeout, should a child process hold them.
	cosignWaitDelay = 5 * time.Second
)

// errSignatureVerification means cosign didn't verify the release's checksums: a bad signature or cosign failing itself.
var errSignatureVerification = errors.New("signature verification failed")

// checkSignature verifies the downloaded checksums at checksumsPath against their release bundle when cosign is set.
// A release without a bundle is unsigned, not an error; an invalid signature is.
func (u updater) checkSignature(checksumsURL, checksumsPath string) error {
	if u.cosign == "" {
		return nil
	}

	bundlePath := checksumsPath + bundleSuffix
	err := downloadFile(u.client, checksumsURL+bundleSuffix, bundlePath)
	if errors.Is(err, errNotFound) {
		u.log.Detail("release is unsigned, verifying checksum only")
		return nil
	}
	if err != nil {
		return fmt.Errorf("downloading signature: %w", err)
	}

	u.log.Detail("verifying signature...")
	return u.verifySignature(checksumsPath, bundlePath)
}

// verifySignature checks the bundle signature of checksumsPath with cosign against the release workflow identity.
func (u updater) verifySignature(checksumsPath, bundlePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), u.cosignTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, u.cosign, // NOSONAR - the user's own cosign from PATH is intended
		"verify-blob",
		"--bundle", bundlePath,
		"--certificate-identity", certIdentity,
		"--certificate-oidc-issuer", certIssuer,
		checksumsPath,
	)
	cmd.WaitDelay = cosignWaitDelay

	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return fmt.Errorf("%w: cosign verify-blob timed out after %s", errSignatureVerification, u.cosignTimeout)
	}
	if err == nil {
		return nil
	}

	if output := strings.TrimSpace(string(out)); output != "" {
		return fmt.Errorf("%w: cosign verify-blob: %w: %s", errSignatureVerification, err, output)
	}
	return fmt.Errorf("%w: cosign verify-blob: %w", errSignatureVerification, err)
}

// cosignPath returns the cosign binary on PATH, or "" when it isn't installed.
func cosignPath() string {
	path, err := exec.LookPath("cosign")
	if err != nil {
		return ""
	}
	return path
}
