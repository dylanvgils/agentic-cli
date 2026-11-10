package docker

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

const (
	// dryRunPortPlaceholder stands in for a published proxy's host port in dry-run output, as Docker only picks it at start.
	dryRunPortPlaceholder = "<published port>"

	// dryRunTokenPlaceholder stands in for a published proxy's auth token in dry-run output, so it is never printed.
	dryRunTokenPlaceholder = "<auth token>"
)

// PublishedProxy is a proxy sidecar published on a host loopback port, for a runtime outside Docker; Stop removes it.
type PublishedProxy struct {
	// Port is the host port on 127.0.0.1 the proxy listens on.
	Port string
	// Token is the password clients send as proxy.AuthUser.
	Token string
	// CAPEM is the CA cert the tool must trust, nil when the proxy doesn't decrypt.
	CAPEM []byte
	// Env is the tool's KEY=VALUE credential placeholders and proxy integration env.
	Env []string

	stop func()
}

// Stop removes the proxy and its volume; safe to defer, including on a zero or dry-run PublishedProxy.
func (p PublishedProxy) Stop() {
	if p.stop != nil {
		p.stop()
	}
}

// StartPublishedProxy starts spec's proxy on a random 127.0.0.1 port with a fresh auth token, logging to proxyLog; a
// dry run only prints it. This process owns it, labelled with owner's runtime.
func (c *Client) StartPublishedProxy(spec sandbox.RunSpec, proxyLog string, owner sandbox.Runtime) (PublishedProxy, error) {
	if _, err := sandbox.ParseRunID(spec.RunID.String()); err != nil {
		return PublishedProxy{}, err
	}

	rs := toolRun{RunSpec: spec, proxyLog: proxyLog}
	h, err := c.newPublishedHandle(rs, owner)
	if err != nil {
		return PublishedProxy{}, err
	}

	if spec.DryRun {
		sandbox.PrintDryRun("egress proxy", "docker", h.createArgs(rs, ""))
		published := h.publishedProxy(dryRunPortPlaceholder)
		published.Token = dryRunTokenPlaceholder
		if h.decrypts() {
			published.CAPEM = []byte(dryRunCAPlaceholder)
		}
		return published, nil
	}

	if err := c.requireLocalContext(); err != nil {
		return PublishedProxy{}, err
	}
	warnOnSweepError(c.sweepOrphanedSidecars(spec.ToolHome))

	// Guard first, so Ctrl-C during startup still tears it down
	interrupt, stopGuard := guardSignals()
	stop := func() {
		_ = c.Teardown(spec.ToolHome, spec.RunID)
		stopGuard()
	}

	published, err := c.startPublished(h, rs, interrupt)
	if err != nil {
		stop()
		return PublishedProxy{}, err
	}
	published.stop = stop
	return published, nil
}

// startPublished starts h's proxy and reads its published port.
func (c *Client) startPublished(h proxyHandle, rs toolRun, interrupt <-chan os.Signal) (PublishedProxy, error) {
	h, err := c.startProxy(h, rs, interrupt)
	if err != nil {
		return PublishedProxy{}, err
	}

	port, err := h.publishedPort()
	if err != nil {
		return PublishedProxy{}, err
	}
	return h.publishedProxy(port), nil
}

// newPublishedHandle returns rs's proxy handle with a fresh auth token, owned by this process's run on owner.
func (c *Client) newPublishedHandle(rs toolRun, owner sandbox.Runtime) (proxyHandle, error) {
	h, err := c.newProxyHandle(rs)
	if err != nil {
		return proxyHandle{}, err
	}

	h.token, err = authToken()
	if err != nil {
		return proxyHandle{}, err
	}
	h.owner = pidOwner(owner, os.Getpid())
	return h, nil
}

// publishedProxy returns h as a PublishedProxy on port.
func (h proxyHandle) publishedProxy(port string) PublishedProxy {
	return PublishedProxy{
		Port:  port,
		Token: h.token,
		CAPEM: h.caPEM,
		Env:   slices.Concat(h.placeholderEnv, h.integrationEnv),
	}
}

// publishedPort returns the host port Docker published the started proxy on.
func (h proxyHandle) publishedPort() (string, error) {
	out, err := h.client.run("port", h.container, tools.ProxyPort+"/tcp")
	if err != nil {
		return "", fmt.Errorf("read the proxy's published port: %w", err)
	}
	return parsePublishedPort(out)
}

// writeTokenFile writes a published proxy's token to a 0600 env file, out of process args; "" when unpublished.
func (h proxyHandle) writeTokenFile() (path string, remove func(), err error) {
	if !h.published() {
		return "", noCleanup, nil
	}

	f, err := os.OpenFile(tokenFilePath(h.id), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", nil, fmt.Errorf("create proxy token file: %w", err)
	}
	remove = func() { _ = os.Remove(f.Name()) }

	_, writeErr := f.WriteString(proxy.EnvAuthToken + "=" + h.token + "\n")
	if closeErr := f.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		remove()
		return "", nil, fmt.Errorf("write proxy token file: %w", writeErr)
	}
	return f.Name(), remove, nil
}

// tokenFilePath returns run id's proxy token file.
func tokenFilePath(id sandbox.RunID) string {
	return filepath.Join(os.TempDir(), "agentic-proxy-token-"+id.String()+".env")
}

// parsePublishedPort returns the port of the first `docker port` line, e.g. "127.0.0.1:49153".
func parsePublishedPort(out string) (string, error) {
	line, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	port := strings.TrimSpace(line[strings.LastIndex(line, ":")+1:])
	if n, err := strconv.Atoi(port); err != nil || n <= 0 || n > 65535 {
		return "", fmt.Errorf("unexpected docker port output %q", strings.TrimSpace(out))
	}
	return port, nil
}

// authToken returns a random hex token for a published proxy's clients.
func authToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate proxy auth token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
