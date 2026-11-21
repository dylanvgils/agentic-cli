package docker

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
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

	// tokenFileName names a published proxy's token file in the run temp dir.
	tokenFileName = "proxy-token.env"
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

	// guard tears the run down; nil when only printed
	guard *runGuard
}

// Stop removes the proxy and its volume; safe to defer, including on a zero or printed PublishedProxy.
func (p PublishedProxy) Stop() {
	p.guard.release()
}

// DryRunPublishedProxy prints spec's published proxy command, starting nothing, and returns it with placeholder values.
func (c *Client) DryRunPublishedProxy(spec sandbox.RunSpec, proxyLog string, owner sandbox.Runtime) (PublishedProxy, error) {
	rs, h, err := c.publishedRun(spec, proxyLog, owner)
	if err != nil {
		return PublishedProxy{}, err
	}

	sandbox.PrintDryRun("egress proxy", "docker", h.createArgs(rs, ""))
	return h.printedProxy(), nil
}

// StartPublishedProxy starts spec's proxy on a random 127.0.0.1 port with a fresh auth token, logging to proxyLog. This
// process owns it, labelled with owner's runtime.
func (c *Client) StartPublishedProxy(spec sandbox.RunSpec, proxyLog string, owner sandbox.Runtime) (PublishedProxy, error) {
	rs, h, err := c.publishedRun(spec, proxyLog, owner)
	if err != nil {
		return PublishedProxy{}, err
	}

	if err := c.RequireLocalContext("microsandbox can't reach a proxy published on it"); err != nil {
		return PublishedProxy{}, err
	}
	warnOnSweepError(c.sweepOrphanedSidecars(spec.ToolHome))

	// Guard first, so Ctrl-C during startup still tears it down
	guard := c.guardRun(spec.ToolHome, spec.RunID, true)
	published, err := c.startPublished(h, rs, guard.signals)
	if err != nil {
		guard.release()
		return PublishedProxy{}, err
	}
	published.guard = guard
	return published, nil
}

// publishedRun returns spec's run and its proxy handle with a fresh auth token, owned by this process's run on owner.
func (c *Client) publishedRun(spec sandbox.RunSpec, proxyLog string, owner sandbox.Runtime) (toolRun, proxyHandle, error) {
	rs, err := newToolRun(spec, proxyLog)
	if err != nil {
		return toolRun{}, proxyHandle{}, err
	}

	h, err := c.newProxyHandle(rs)
	if err != nil {
		return toolRun{}, proxyHandle{}, err
	}

	h.token, err = authToken()
	if err != nil {
		return toolRun{}, proxyHandle{}, err
	}
	h.owner = pidOwner(owner, os.Getpid())
	return rs, h, nil
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

// printedProxy returns h as a dry run's PublishedProxy, with placeholders for what only exists once started.
func (h proxyHandle) printedProxy() PublishedProxy {
	published := h.publishedProxy(dryRunPortPlaceholder)
	published.Token = dryRunTokenPlaceholder
	if h.decrypts() {
		published.CAPEM = []byte(dryRunCAPlaceholder)
	}
	return published
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

	f, err := sandbox.CreateRunFile(h.id, tokenFileName)
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
