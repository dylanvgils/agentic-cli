package docker

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dylanvgils/agentic-cli/internal/certs"
	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
)

// ProxyCAEnvName points Node and Bun at the proxy CA, since they ignore the system bundle.
const ProxyCAEnvName = "NODE_EXTRA_CA_CERTS"

// loadSystemCABundle is a test-stubbable indirection into systemCABundle.
var loadSystemCABundle = systemCABundle

const (
	// proxyDirName is the subdirectory under the agentic home holding the per-run trust files the tool mounts.
	proxyDirName = "proxy"

	// proxyBundleFile is the image's system bundle plus the proxy CA, mounted over systemCABundlePath.
	proxyBundleFile = "bundle.crt"

	// proxyCAFile is the proxy CA alone, mounted at proxyCAContainerPath for ProxyCAEnvName.
	proxyCAFile = "proxy-ca.pem"

	// proxyCAContainerPath is where the tool container sees proxyCAFile.
	proxyCAContainerPath = "/run/agentic/proxy-ca.pem"
)

// prepareTrust issues a CA scoped to the credential hosts and writes the tool's trust files for it; a zero CA without credentials.
func (h proxyHandle) prepareTrust(rs RunSpec) (certs.CA, error) {
	if len(h.credentials) == 0 {
		return certs.CA{}, nil
	}

	ca, err := certs.NewScopedCA("agentic proxy CA", proxy.CredentialHosts(h.credentials))
	if err != nil {
		return certs.CA{}, err
	}
	if err := h.writeTrustFiles(rs.ToolHome, rs.Image, ca.CertPEM()); err != nil {
		_ = os.RemoveAll(h.runDir)
		return certs.CA{}, err
	}
	return ca, nil
}

// writeTrustFiles writes the tool-facing trust files for caPEM into the run dir; only public certs, the CA key stays in the proxy.
func (h proxyHandle) writeTrustFiles(toolHome, image string, caPEM []byte) error {
	system, err := loadSystemCABundle(toolHome, image)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(h.runDir, 0o700); err != nil {
		return fmt.Errorf("create proxy run dir: %w", err)
	}

	bundle := append([]byte{}, system...)
	if len(bundle) > 0 && bundle[len(bundle)-1] != '\n' {
		bundle = append(bundle, '\n')
	}
	bundle = append(bundle, caPEM...)

	if err := os.WriteFile(filepath.Join(h.runDir, proxyBundleFile), bundle, 0o644); err != nil {
		return fmt.Errorf("write proxy CA bundle: %w", err)
	}
	if err := os.WriteFile(filepath.Join(h.runDir, proxyCAFile), caPEM, 0o644); err != nil {
		return fmt.Errorf("write proxy CA: %w", err)
	}
	return nil
}

// trustArgs returns the tool's mounts and env for trusting the proxy CA; nil without credentials.
func (h proxyHandle) trustArgs() []string {
	if len(h.credentials) == 0 {
		return nil
	}

	bundle := mount.NormalizeMountSpec(filepath.Join(h.runDir, proxyBundleFile) + ":" + systemCABundlePath)
	ca := mount.NormalizeMountSpec(filepath.Join(h.runDir, proxyCAFile) + ":" + proxyCAContainerPath)
	return []string{
		arg("volume", bundle+":ro"),
		arg("volume", ca+":ro"),
		arg("env", ProxyCAEnvName+"="+proxyCAContainerPath),
	}
}
