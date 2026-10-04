package tools

import (
	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
)

const (
	// ProxyCAEnvName carries the proxy CA cert PEM into the tool container; set by agentic when proxy credentials are configured.
	ProxyCAEnvName = "AGENTIC_PROXY_CA"

	// ProxyTrustLabel marks a tool image whose entrypoint runs proxyTrustBlock; older images ignore ProxyCAEnvName.
	ProxyTrustLabel = "agentic.proxy-trust"

	// proxyCABundlePath is the tmpfs file the entrypoint writes the system bundle plus the proxy CA to.
	proxyCABundlePath = "/tmp/agentic-ca-bundle.crt"

	// systemCABundlePath is the Debian system trust bundle the proxy CA is appended to.
	systemCABundlePath = "/etc/ssl/certs/ca-certificates.crt"
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
	lines := []string{
		`if [[ -n "${` + ProxyCAEnvName + `:-}" ]]; then`,
		`  { cat ` + systemCABundlePath + `; printf '\n%s\n' "$` + ProxyCAEnvName + `"; } > ` + proxyCABundlePath,
	}
	for _, name := range ProxyTrustEnvNames {
		lines = append(lines, `  export `+name+`=`+proxyCABundlePath)
	}
	lines = append(lines, "  unset "+ProxyCAEnvName, "fi")

	return df.Block{Comment: "Trust the egress proxy's CA for credential hosts", Lines: lines}
}

// proxyTrustLabel marks the image as running proxyTrustBlock; add it next to the entrypoint that does.
func proxyTrustLabel() df.Label {
	return df.Label{Key: ProxyTrustLabel, Value: "true"}
}
