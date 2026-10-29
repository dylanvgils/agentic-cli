package run

import (
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
)

// requiredCapabilities returns what a run with flags and rc needs; the proxy counts only when asked for, not when credentials force it.
func requiredCapabilities(flags Flags, rc *config.AgenticRC) sandbox.Capabilities {
	proxyMode := resolve.RequestedProxyMode(flags.Proxy, rc)
	return sandbox.Capabilities{
		Proxy:               proxyMode.Enabled(),
		ProxyMonitor:        proxyMode == sandbox.ProxyMonitor,
		Dind:                resolve.DindEnabled(flags.Dind, rc),
		CredentialInjection: len(rc.Run.Proxy.Credentials) > 0,
		PathRules:           len(rc.Run.Proxy.AllowedPaths) > 0 && (proxyMode.Enabled() || len(rc.Run.Proxy.Credentials) > 0),
		ProxyIntegrations:   len(rc.Run.Proxy.Integrations) > 0 && (proxyMode.Enabled() || len(rc.Run.Proxy.Credentials) > 0),
		PidsLimit:           true,
	}
}

// specCapabilities returns the security features rs relies on its runtime to enforce.
func specCapabilities(rs sandbox.RunSpec) sandbox.Capabilities {
	return sandbox.Capabilities{
		Proxy:               rs.Proxy.Mode.Enabled(),
		ProxyMonitor:        rs.Proxy.Mode == sandbox.ProxyMonitor,
		CredentialInjection: len(rs.Credentials) > 0,
		PathRules:           rs.Proxy.Mode.Enabled() && rs.Proxy.HasPathRules(),
	}
}

// checkCapabilities refuses a run that needs a security feature offered lacks and warns about a missing Docker daemon; a
// missing pids limit is silent.
func checkCapabilities(req Request, offered sandbox.Capabilities) error {
	missing := offered.Missing(requiredCapabilities(req.Flags, req.Project.RC))

	var refused []string
	if missing.Proxy {
		refused = append(refused, fmt.Sprintf("the egress proxy (%s)", proxySource(req)))
	} else if missing.ProxyMonitor {
		refused = append(refused, fmt.Sprintf("proxy monitor mode (%s)", proxySource(req)))
	}
	if missing.CredentialInjection {
		refused = append(refused, fmt.Sprintf("proxy credentials (%s)", credentialsSource(req)))
	}
	if len(refused) > 0 {
		hint := "use --runtime docker"
		if missing.ProxyMonitor && !missing.Proxy {
			hint = "pass --proxy to enforce the allowlist instead, or " + hint
		}
		if missing.Proxy {
			hint = "pass --no-proxy to run without the proxy, or " + hint
		}
		return fmt.Errorf("%s does not support %s; %s", runtimeSource(req), strings.Join(refused, " or "), hint)
	}

	if missing.PathRules {
		logging.Warnf("%s can't limit hosts to paths, so it allows the hosts of %s on every path; use --runtime docker to limit them", runtimeSource(req), pathRulesSource(req))
	}
	if missing.ProxyIntegrations {
		logging.Warnf("%s has no proxy sidecar, so it ignores %s; tools reach allowed hosts directly", runtimeSource(req), integrationsSource(req))
	}
	if missing.Dind {
		logging.Warnf("%s has no Docker daemon, so it ignores %s", runtimeSource(req), dindSource(req))
	}
	return nil
}

// checkSpec refuses rs if it relies on a security feature offered lacks; a backstop for checkCapabilities, which refuses the
// request before any prompt, so a spec built some other way can't run with a control silently dropped.
func checkSpec(rs sandbox.RunSpec, offered sandbox.Capabilities) error {
	missing := offered.Missing(specCapabilities(rs))

	var refused []string
	if missing.Proxy {
		refused = append(refused, "the egress proxy")
	} else if missing.ProxyMonitor {
		refused = append(refused, "proxy monitor mode")
	}
	if missing.CredentialInjection {
		refused = append(refused, "credential injection")
	}
	if missing.PathRules {
		refused = append(refused, "path rules")
	}
	if len(refused) > 0 {
		return fmt.Errorf("the runtime can't enforce %s that the run asks for", strings.Join(refused, " or "))
	}
	return nil
}

// runtimeSource names what picks req's runtime: --runtime, or [run] runtime and the innermost layer setting it.
func runtimeSource(req Request) string {
	if req.Flags.Runtime != "" {
		return "--runtime " + string(req.Flags.Runtime)
	}
	key := fmt.Sprintf("[run] runtime = %q", req.Project.RC.Run.Runtime)
	return layerSource(req.Project.Layers, key, func(rc *config.AgenticRC) bool { return rc.Run.Runtime != "" })
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

// pathRulesSource names the innermost layer that declares proxy path rules.
func pathRulesSource(req Request) string {
	return layerSource(req.Project.Layers, "[[run.proxy.allowed_paths]]", func(rc *config.AgenticRC) bool { return len(rc.Run.Proxy.AllowedPaths) > 0 })
}

// integrationsSource names the innermost layer setting proxy integrations.
func integrationsSource(req Request) string {
	return layerSource(req.Project.Layers, "[run.proxy] integrations", func(rc *config.AgenticRC) bool { return len(rc.Run.Proxy.Integrations) > 0 })
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
