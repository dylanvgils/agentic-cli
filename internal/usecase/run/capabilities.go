package run

import (
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
)

// featurePolicy is how a run handles a feature its runtime lacks.
type featurePolicy int

const (
	// policyRefuse refuses the run: the feature is a security control it relies on.
	policyRefuse featurePolicy = iota
	// policyWarn warns and runs: the runtime ignores the setting.
	policyWarn
)

// featureRule says how a run handles one Features field its runtime lacks.
type featureRule struct {
	has      func(sandbox.Features) bool // reads the field
	policy   featurePolicy
	shadowed func(sandbox.Features) bool          // optional: a broader missing feature is reported instead
	refused  func(Request) string                 // refuse: names the feature and its source
	hint     string                               // refuse: prefix for "use --runtime docker"
	refuse   func(Request, sandbox.Runtime) error // refuse: an error of its own that replaces the joined one
	warn     func(Request) string                 // warn: the whole message
	spec     string                               // checkSpec's name; "" when checkSpec skips it
}

// featureRules covers every Features field, in field order.
var featureRules = []featureRule{
	{
		has:     func(f sandbox.Features) bool { return f.Proxy },
		policy:  policyRefuse,
		refused: func(req Request) string { return fmt.Sprintf("the egress proxy (%s)", proxySource(req)) },
		hint:    "pass --no-proxy to run without the proxy, or ",
		spec:    "the egress proxy",
	},
	{
		has:      func(f sandbox.Features) bool { return f.ProxyMonitor },
		policy:   policyRefuse,
		shadowed: func(f sandbox.Features) bool { return f.Proxy },
		refused:  func(req Request) string { return fmt.Sprintf("proxy monitor mode (%s)", proxySource(req)) },
		hint:     "pass --proxy to enforce the allowlist instead, or ",
		spec:     "proxy monitor mode",
	},
	{
		has:    func(f sandbox.Features) bool { return f.Dind },
		policy: policyWarn,
		warn: func(req Request) string {
			return fmt.Sprintf("%s has no Docker daemon, so it ignores %s", runtimeSource(req), dindSource(req))
		},
	},
	{
		has:      func(f sandbox.Features) bool { return f.DindLimits },
		policy:   policyWarn,
		shadowed: func(f sandbox.Features) bool { return f.Dind },
		warn: func(req Request) string {
			return fmt.Sprintf("%s ignores the Docker daemon limits ([run.dind] or --dind-*); dockerd shares the VM's CPUs and memory", runtimeSource(req))
		},
	},
	{
		has:     func(f sandbox.Features) bool { return f.CredentialInjection },
		policy:  policyRefuse,
		refused: func(req Request) string { return fmt.Sprintf("proxy credentials (%s)", credentialsSource(req)) },
		spec:    "credential injection",
	},
	{
		has:    func(f sandbox.Features) bool { return f.PathRules },
		policy: policyWarn,
		warn: func(req Request) string {
			return fmt.Sprintf("%s can't limit hosts to paths, so it allows the hosts of %s on every path; use --runtime docker to limit them", runtimeSource(req), pathRulesSource(req))
		},
		spec: "path rules",
	},
	{
		has:    func(f sandbox.Features) bool { return f.ProxyIntegrations },
		policy: policyWarn,
		warn: func(req Request) string {
			return fmt.Sprintf("%s ignores %s; it can't route those tools through the proxy", runtimeSource(req), integrationsSource(req))
		},
	},
	{
		has:    func(f sandbox.Features) bool { return f.PidsLimit },
		policy: policyWarn,
		warn: func(req Request) string {
			return fmt.Sprintf("%s ignores the process limit ([run] pids_limit or --pids-limit); the VM has its own kernel", runtimeSource(req))
		},
	},
	{
		has:    func(f sandbox.Features) bool { return f.PortsWithProxy },
		policy: policyRefuse,
		refuse: refusePortsWithProxy,
		spec:   "published ports with the egress proxy",
	},
}

// reported reports whether r's feature is in missing and no broader missing feature covers it.
func (r featureRule) reported(missing sandbox.Features) bool {
	return r.has(missing) && (r.shadowed == nil || !r.shadowed(missing))
}

// requiredFeatures returns what a run with flags and rc needs; the proxy counts only when asked for, not when credentials force it.
func requiredFeatures(flags Flags, rc *config.AgenticRC) sandbox.Features {
	proxyMode := resolve.RequestedProxyMode(flags.Proxy, rc)
	dind := resolve.DindEnabled(flags.Dind, rc)
	ports := portsSet(flags, rc)
	return sandbox.Features{
		Proxy:               proxyMode.Enabled(),
		ProxyMonitor:        proxyMode == sandbox.EgressMonitor,
		Dind:                dind,
		DindLimits:          dind && dindLimitsSet(flags, rc),
		CredentialInjection: len(rc.Run.Proxy.Credentials) > 0,
		PathRules:           len(rc.Run.Proxy.AllowedPaths) > 0 && (proxyMode.Enabled() || len(rc.Run.Proxy.Credentials) > 0),
		ProxyIntegrations:   len(resolve.ProxyIntegrations(flags.Proxy, rc)) > 0 && (proxyMode.Enabled() || len(rc.Run.Proxy.Credentials) > 0),
		PidsLimit:           flags.Limits.PidsLimit != "" || rc.Run.PidsLimit != "",
		PortsWithProxy:      ports && (proxyMode.Enabled() || len(rc.Run.Proxy.Credentials) > 0),
	}
}

// specFeatures returns the security features rs relies on its runtime to enforce.
func specFeatures(rs sandbox.RunSpec) sandbox.Features {
	return sandbox.Features{
		Proxy:               rs.Egress.Mode.Enabled(),
		ProxyMonitor:        rs.Egress.Mode == sandbox.EgressMonitor,
		CredentialInjection: len(rs.Credentials) > 0,
		PathRules:           rs.Egress.Mode.Enabled() && rs.Egress.HasPathRules(),
		PortsWithProxy:      rs.Egress.Mode.Enabled() && len(rs.Ports) > 0,
	}
}

// checkCapabilities refuses missing security features and warns about ignored settings.
func checkCapabilities(req Request, runtime sandbox.Runtime, offered sandbox.Features) error {
	missing := offered.Missing(requiredFeatures(req.Flags, req.Project.RC))
	if err := refuseMissing(req, runtime, missing); err != nil {
		return err
	}

	warnIgnored(req, missing)
	return nil
}

// checkSpec refuses rs if it relies on a security feature offered lacks; a backstop for checkCapabilities, which refuses the
// request before any prompt, so a spec built some other way can't run with a control silently dropped.
func checkSpec(rs sandbox.RunSpec, offered sandbox.Features) error {
	missing := offered.Missing(specFeatures(rs))

	var refused []string
	for _, r := range featureRules {
		if r.spec != "" && r.reported(missing) {
			refused = append(refused, r.spec)
		}
	}
	if len(refused) > 0 {
		return fmt.Errorf("the runtime can't enforce %s that the run asks for", strings.Join(refused, " or "))
	}
	return nil
}

// refuseMissing errors on missing security features, with a hint; a rule with an error of its own wins.
func refuseMissing(req Request, runtime sandbox.Runtime, missing sandbox.Features) error {
	var refused []string
	var hint string
	for _, r := range featureRules {
		if r.policy != policyRefuse || !r.reported(missing) {
			continue
		}
		if r.refuse != nil {
			return r.refuse(req, runtime)
		}
		refused = append(refused, r.refused(req))
		if hint == "" {
			hint = r.hint
		}
	}
	if len(refused) > 0 {
		return fmt.Errorf("%s does not support %s; %suse --runtime docker", runtimeSource(req), strings.Join(refused, " or "), hint)
	}
	return nil
}

// warnIgnored warns about settings the runtime ignores.
func warnIgnored(req Request, missing sandbox.Features) {
	for _, r := range featureRules {
		if r.policy == policyWarn && r.reported(missing) {
			logging.Warn(r.warn(req))
		}
	}
}

// runtimeSource names what picks req's runtime: --runtime, or [run] runtime and the innermost layer setting it.
func runtimeSource(req Request) string {
	if req.Flags.Runtime != "" {
		return "--runtime " + string(req.Flags.Runtime)
	}
	key := fmt.Sprintf("[run] runtime = %q", req.Project.RC.Run.Runtime)
	return layerSource(req.Project.Layers, key, func(rc *config.AgenticRC) bool { return rc.Run.Runtime != "" })
}

// proxyBackendSource names what picks req's proxy backend: --proxy-backend, or [run.proxy] backend and the innermost layer
// setting it.
func proxyBackendSource(req Request) string {
	if req.Flags.Proxy.Backend != "" {
		return "--proxy-backend " + req.Flags.Proxy.Backend
	}
	key := fmt.Sprintf("[run.proxy] backend = %q", req.Project.RC.Run.Proxy.Backend)
	return layerSource(req.Project.Layers, key, func(rc *config.AgenticRC) bool { return rc.Run.Proxy.Backend != "" })
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

// integrationsSource names what sets proxy integrations in req: --proxy-integration, or the innermost layer setting them.
func integrationsSource(req Request) string {
	if len(req.Flags.Proxy.Integrations) > 0 {
		return "--proxy-integration"
	}
	return layerSource(req.Project.Layers, "[run.proxy] integrations", func(rc *config.AgenticRC) bool { return len(rc.Run.Proxy.Integrations) > 0 })
}

// portsSource names what sets ports in req: --port, or [run] ports and the innermost layer setting them.
func portsSource(req Request) string {
	if len(req.Flags.Ports) > 0 {
		return "--port"
	}
	return layerSource(req.Project.Layers, "[run] ports", func(rc *config.AgenticRC) bool { return len(rc.Run.Ports) > 0 })
}

// refusePortsWithProxy errors on ports in a run of req on runtime whose egress goes through the proxy, naming what turns
// it on.
func refusePortsWithProxy(req Request, runtime sandbox.Runtime) error {
	cause, hint := credentialsSource(req), "use --runtime microsandbox"
	if resolve.RequestedProxyMode(req.Flags.Proxy, req.Project.RC).Enabled() {
		cause, hint = proxySource(req), "pass --no-proxy or "+hint
	}
	return fmt.Errorf("%s can't be used with the egress proxy (%s) on the %s runtime, as its run network has no route to the host; %s", portsSource(req), cause, runtime, hint)
}

// dindLimitsSet reports whether flags or rc set Docker daemon limits.
func dindLimitsSet(flags Flags, rc *config.AgenticRC) bool {
	return flags.DindLimits != (sandbox.ResourceLimits{}) || rc.Run.Dind.RCLimits != (config.RCLimits{})
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
