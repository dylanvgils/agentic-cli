package tools

import (
	df "github.com/dylanvgils/agentic-cli/internal/image/dockerfile"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

const (
	// ProxyCAEnvName carries the proxy CA cert PEM into the tool container; set by agentic when proxy credentials are configured.
	ProxyCAEnvName = sandbox.ProxyCAEnvName

	// ProxyTrustLabel marks a tool image whose entrypoint runs proxyTrustBlock; older images ignore ProxyCAEnvName.
	ProxyTrustLabel = "agentic.proxy-trust"

	// proxyCABundlePath is the tmpfs file the entrypoint writes the system bundle plus the proxy CA to.
	proxyCABundlePath = "/tmp/agentic-ca-bundle.crt"

	// systemCABundlePath is the Debian system trust bundle the proxy CA is appended to.
	systemCABundlePath = "/etc/ssl/certs/ca-certificates.crt"

	// msbCAPath is where msb puts its TLS inspection CA in the guest.
	msbCAPath = "/.msb/tls/ca.pem"

	// javaTrustStorePath is the tmpfs copy of the JDK's cacerts the entrypoint adds the proxy and msb CAs to.
	javaTrustStorePath = "/tmp/agentic-java-cacerts"

	// javaTrustStorePassword is the JDK's default cacerts password; the store holds only public certs.
	javaTrustStorePassword = "changeit"
)

// ProxyTrustEnvNames are the vars the entrypoint points at the bundle. The read-only, non-root container can't replace
// systemCABundlePath, and each client reads its own override:
//   - SSL_CERT_FILE: OpenSSL and Go (e.g. gh)
//   - CURL_CA_BUNDLE: curl
//   - GIT_SSL_CAINFO: git over HTTPS
//   - REQUESTS_CA_BUNDLE: Python requests and pip, which bundle their own roots
//   - NODE_EXTRA_CA_CERTS: Node and Bun (Claude Code, Copilot, OpenCode), which ignore the system bundle
var ProxyTrustEnvNames = []string{"SSL_CERT_FILE", "CURL_CA_BUNDLE", "GIT_SSL_CAINFO", "REQUESTS_CA_BUNDLE", "NODE_EXTRA_CA_CERTS"}

// proxyTrustBlock is the entrypoint block that builds a CA bundle trusting ProxyCAEnvName and points ProxyTrustEnvNames at it.
func proxyTrustBlock() df.Block {
	return scriptBlock("proxytrust.sh")
}

// trustBlocks are the entrypoint blocks that trust the proxy CA, adding Java's trust store only on the java extra.
func trustBlocks(img ImageInfo) []df.Block {
	if img.HasExtra("java") {
		return []df.Block{javaTrustBlock(), proxyTrustBlock()}
	}
	return []df.Block{proxyTrustBlock()}
}

// javaTrustBlock is the entrypoint block that points Java, which ignores the PEM bundle vars, at a copy of the JDK's
// cacerts with the proxy and msb CAs added; it must run before proxyTrustBlock unsets ProxyCAEnvName.
func javaTrustBlock() df.Block {
	return scriptBlock("javatrust.sh")
}

// proxyTrustLabel marks the image as running proxyTrustBlock; add it next to the entrypoint that does.
func proxyTrustLabel() df.Label {
	return df.Label{Key: ProxyTrustLabel, Value: "true"}
}
