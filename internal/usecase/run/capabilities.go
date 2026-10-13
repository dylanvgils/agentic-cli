package run

import (
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
)

// capabilities returns what runtime offers; an unknown runtime offers nothing.
func (s *Service) capabilities(runtime Runtime) sandbox.Capabilities {
	switch runtime {
	case RuntimeDocker, "":
		return s.docker.Capabilities()
	case RuntimeMicrosandbox:
		return s.microsandbox.Capabilities()
	default:
		return sandbox.Capabilities{}
	}
}

// checkRuntime refuses a run req's runtime can't do safely and warns about what it drops, then runs that runtime's own checks.
func (s *Service) checkRuntime(req Request) error {
	if err := checkCapabilities(req, s.capabilities(req.Flags.Runtime)); err != nil {
		return err
	}

	if req.Flags.Runtime == RuntimeMicrosandbox {
		return s.checkMicrosandbox(req)
	}
	return nil
}

// requiredCapabilities returns what a run with flags and rc needs; the proxy counts only when asked for, not when credentials force it.
func requiredCapabilities(flags Flags, rc *config.AgenticRC) sandbox.Capabilities {
	return sandbox.Capabilities{
		Proxy:               resolve.RequestedProxyMode(flags.Proxy, rc).Enabled(),
		Dind:                resolve.DindEnabled(flags.Dind, rc),
		CredentialInjection: len(rc.Run.Proxy.Credentials) > 0,
		PidsLimit:           true,
	}
}

// checkCapabilities refuses a run that needs a security feature offered lacks and warns about a missing Docker daemon; a
// missing pids limit is silent.
func checkCapabilities(req Request, offered sandbox.Capabilities) error {
	missing := offered.Missing(requiredCapabilities(req.Flags, req.Project.RC))

	var refused []string
	if missing.Proxy {
		refused = append(refused, fmt.Sprintf("the egress proxy (%s)", proxySource(req)))
	}
	if missing.CredentialInjection {
		refused = append(refused, fmt.Sprintf("proxy credentials (%s)", credentialsSource(req)))
	}
	if len(refused) > 0 {
		hint := "use --runtime docker"
		if missing.Proxy {
			hint = "pass --no-proxy to run without the proxy, or " + hint
		}
		return fmt.Errorf("--runtime %s does not support %s; %s", req.Flags.Runtime, strings.Join(refused, " or "), hint)
	}

	if missing.Dind {
		logging.Warnf("--runtime %s has no Docker daemon, so it ignores %s", req.Flags.Runtime, dindSource(req))
	}
	return nil
}

// proxySource names what asks for the proxy in req: its flag, or the [run.proxy] key and the innermost layer setting it.
func proxySource(req Request) string {
	switch {
	case req.Flags.Proxy.MonitorFlag:
		return "--proxy-monitor"
	case req.Flags.Proxy.ProxyFlag:
		return "--proxy"
	case req.Project.RC.Run.Proxy.Mode == config.ModeMonitor:
		return layerSource(req.Project.Layers, "[run.proxy] mode", func(rc *config.AgenticRC) bool { return rc.Run.Proxy.Mode == config.ModeMonitor })
	default:
		return layerSource(req.Project.Layers, "[run.proxy] enabled", func(rc *config.AgenticRC) bool { return rc.Run.Proxy.Enabled != nil })
	}
}

// dindSource names what asks for the Docker daemon in req: --dind, or [run.dind] enabled and the innermost layer setting it.
func dindSource(req Request) string {
	if req.Flags.Dind.DindFlag {
		return "--dind"
	}
	return layerSource(req.Project.Layers, "[run.dind] enabled", func(rc *config.AgenticRC) bool { return rc.Run.Dind.Enabled != nil })
}

// credentialsSource names the innermost layer that declares proxy credentials.
func credentialsSource(req Request) string {
	return layerSource(req.Project.Layers, "[[run.proxy.credentials]]", func(rc *config.AgenticRC) bool { return len(rc.Run.Proxy.Credentials) > 0 })
}

// layerSource returns key plus the path of the innermost layer where set holds, or key alone when none does.
func layerSource(layers []config.RCLayer, key string, set func(*config.AgenticRC) bool) string {
	for i := len(layers) - 1; i >= 0; i-- {
		if layers[i].RC != nil && set(layers[i].RC) {
			return fmt.Sprintf("%s in %s", key, layers[i].Path)
		}
	}
	return key
}
