package run

import (
	"reflect"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config/agenticrc"
	"github.com/dylanvgils/agentic-cli/internal/config/precedence"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_requiredFeatures(t *testing.T) {
	enabled := true
	proxyFlag := precedence.ProxyInput{ProxyFlag: true}
	paths := []agenticrc.AllowedPath{{Host: "raw.example.test", Paths: []string{"/a"}}}
	creds := []agenticrc.Credential{{Preset: "github", Secret: "/example.test/token"}}

	for _, tc := range []struct {
		name  string
		flags Flags
		rc    agenticrc.Config
		want  sandbox.Features
	}{
		{
			name: "a bare run needs nothing",
		},
		{
			name:  "the pids limit flag asks for limit support",
			flags: Flags{Limits: sandbox.ResourceLimits{PidsLimit: "50"}},
			want:  sandbox.Features{PidsLimit: true},
		},
		{
			name: "a config pids limit asks for limit support",
			rc:   agenticrc.Config{Run: agenticrc.Run{Limits: agenticrc.Limits{PidsLimit: "50"}}},
			want: sandbox.Features{PidsLimit: true},
		},
		{
			name:  "flags ask for the proxy and dind",
			flags: Flags{Proxy: precedence.ProxyInput{MonitorFlag: true}, Dind: precedence.DindInput{DindFlag: true}},
			want:  sandbox.Features{Proxy: true, ProxyMonitor: true, Dind: true},
		},
		{
			name: "config asks for the proxy and dind",
			rc:   agenticrc.Config{Run: agenticrc.Run{Proxy: agenticrc.Proxy{Enabled: &enabled}, Dind: agenticrc.Dind{Enabled: &enabled}}},
			want: sandbox.Features{Proxy: true, Dind: true},
		},
		{
			name: "off switches drop the need",
			flags: Flags{
				Proxy: precedence.ProxyInput{NoProxy: true},
				Dind:  precedence.DindInput{NoDindFlag: true},
			},
			rc:   agenticrc.Config{Run: agenticrc.Run{Proxy: agenticrc.Proxy{Enabled: &enabled}, Dind: agenticrc.Dind{Enabled: &enabled}}},
			want: sandbox.Features{},
		},
		{
			name:  "dind limits ask for limit support with dind",
			flags: Flags{Dind: precedence.DindInput{DindFlag: true}, DindLimits: sandbox.ResourceLimits{Memory: "8g"}},
			want:  sandbox.Features{Dind: true, DindLimits: true},
		},
		{
			name: "dind limits without dind have nothing to limit",
			rc:   agenticrc.Config{Run: agenticrc.Run{Dind: agenticrc.Dind{Limits: agenticrc.Limits{Memory: "8g"}}}},
			want: sandbox.Features{},
		},
		{
			name: "credentials alone don't ask for the proxy",
			rc:   agenticrc.Config{Run: agenticrc.Run{Proxy: agenticrc.Proxy{Credentials: creds}}},
			want: sandbox.Features{CredentialInjection: true},
		},
		{
			name: "path rules without the proxy or credentials need nothing",
			rc:   agenticrc.Config{Run: agenticrc.Run{Proxy: agenticrc.Proxy{AllowedPaths: paths}}},
			want: sandbox.Features{},
		},
		{
			name:  "path rules ask for path rule support with the proxy",
			flags: Flags{Proxy: proxyFlag},
			rc:    agenticrc.Config{Run: agenticrc.Run{Proxy: agenticrc.Proxy{AllowedPaths: paths}}},
			want:  sandbox.Features{Proxy: true, PathRules: true},
		},
		{
			name: "path rules ask for path rule support with credentials",
			rc:   agenticrc.Config{Run: agenticrc.Run{Proxy: agenticrc.Proxy{AllowedPaths: paths, Credentials: creds}}},
			want: sandbox.Features{CredentialInjection: true, PathRules: true},
		},
		{
			name: "proxy integrations without the proxy or credentials need nothing",
			rc:   agenticrc.Config{Run: agenticrc.Run{Proxy: agenticrc.Proxy{Integrations: []string{"maven"}}}},
			want: sandbox.Features{},
		},
		{
			name:  "proxy integrations ask for integration support with the proxy",
			flags: Flags{Proxy: proxyFlag},
			rc:    agenticrc.Config{Run: agenticrc.Run{Proxy: agenticrc.Proxy{Integrations: []string{"maven"}}}},
			want:  sandbox.Features{Proxy: true, ProxyIntegrations: true},
		},
		{
			name: "proxy integrations ask for integration support with credentials",
			rc:   agenticrc.Config{Run: agenticrc.Run{Proxy: agenticrc.Proxy{Integrations: []string{"maven"}, Credentials: creds}}},
			want: sandbox.Features{CredentialInjection: true, ProxyIntegrations: true},
		},
		{
			name:  "the integration flag asks for integration support with the proxy",
			flags: Flags{Proxy: precedence.ProxyInput{ProxyFlag: true, Integrations: []string{"maven"}}},
			want:  sandbox.Features{Proxy: true, ProxyIntegrations: true},
		},
		{
			name:  "ports without the proxy or credentials need nothing",
			flags: Flags{Ports: []string{"3000"}},
			want:  sandbox.Features{},
		},
		{
			name:  "ports ask for ports with the proxy when it's on",
			flags: Flags{Proxy: proxyFlag, Ports: []string{"3000"}},
			want:  sandbox.Features{Proxy: true, PortsWithProxy: true},
		},
		{
			name: "config ports ask for ports with the proxy with credentials",
			rc:   agenticrc.Config{Run: agenticrc.Run{Ports: []string{"3000"}, Proxy: agenticrc.Proxy{Credentials: creds}}},
			want: sandbox.Features{CredentialInjection: true, PortsWithProxy: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			need := requiredFeatures(tc.flags, &tc.rc)

			// Assert
			assert.Equal(t, tc.want, need)
		})
	}
}

func Test_featureRules(t *testing.T) {
	typ := reflect.TypeFor[sandbox.Features]()
	for i := range typ.NumField() {
		field := typ.Field(i)
		t.Run(field.Name, func(t *testing.T) {
			// Arrange
			require.Equal(t, reflect.Bool, field.Type.Kind(), "featureRules only reads bool fields")
			var only sandbox.Features
			reflect.ValueOf(&only).Elem().Field(i).SetBool(true)

			// Act
			matches := 0
			for _, r := range featureRules {
				if r.has(only) {
					matches++
				}
			}

			// Assert
			assert.Equal(t, 1, matches, "featureRules needs exactly one entry for %s", field.Name)
		})
	}
}

func Test_checkCapabilities(t *testing.T) {
	msb := sandbox.Features{Proxy: true, CredentialInjection: true}
	noProxy := sandbox.Features{CredentialInjection: true}

	for _, tc := range []struct {
		name    string
		req     Request
		offered sandbox.Features
		wantErr string
	}{
		{
			name:    "a missing proxy is an error with the opt-out",
			req:     msbRequest(Flags{Proxy: precedence.ProxyInput{ProxyFlag: true}}, nil),
			offered: noProxy,
			wantErr: "--runtime microsandbox does not support the egress proxy (--proxy); pass --no-proxy to run without the proxy, or use --runtime docker",
		},
		{
			name:    "a missing proxy asked for in config names the layer",
			req:     msbRequest(Flags{}, func(rc *agenticrc.Config) { enabled := true; rc.Run.Proxy.Enabled = &enabled }),
			offered: noProxy,
			wantErr: "--runtime microsandbox does not support the egress proxy ([run.proxy] enabled in /example.test/project/.agenticrc.toml); pass --no-proxy to run without the proxy, or use --runtime docker",
		},
		{
			name:    "a missing monitor mode is an error pointing to --proxy",
			req:     msbRequest(Flags{Proxy: precedence.ProxyInput{MonitorFlag: true}}, nil),
			offered: msb,
			wantErr: "--runtime microsandbox does not support proxy monitor mode (--proxy-monitor); pass --proxy to enforce the allowlist instead, or use --runtime docker",
		},
		{
			name:    "ports with the proxy flag are an error with the opt-out",
			req:     dockerRequest(Flags{Proxy: precedence.ProxyInput{ProxyFlag: true}, Ports: []string{"3000"}}, nil),
			wantErr: "--port can't be used with the egress proxy (--proxy) on the docker runtime, as its run network has no route to the host; pass --no-proxy or use --runtime microsandbox",
		},
		{
			name: "config ports with credentials name both layers",
			req: dockerRequest(Flags{}, func(rc *agenticrc.Config) {
				rc.Run.Ports = []string{"3000"}
				rc.Run.Proxy.Credentials = []agenticrc.Credential{{Preset: "github", Secret: "/example.test/token"}}
			}),
			wantErr: "[run] ports in /example.test/project/.agenticrc.toml can't be used with the egress proxy ([[run.proxy.credentials]] in /example.test/project/.agenticrc.toml) on the docker runtime, as its run network has no route to the host; use --runtime microsandbox",
		},
		{
			name:    "a missing monitor mode asked for in config names the layer",
			req:     msbRequest(Flags{}, func(rc *agenticrc.Config) { rc.Run.Proxy.Mode = agenticrc.ModeMonitor }),
			offered: msb,
			wantErr: "--runtime microsandbox does not support proxy monitor mode ([run.proxy] mode in /example.test/project/.agenticrc.toml); pass --proxy to enforce the allowlist instead, or use --runtime docker",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			err := checkCapabilities(tc.req, sandbox.RuntimeDocker, tc.offered)

			// Assert
			assert.EqualError(t, err, tc.wantErr)
		})
	}

	t.Run("missing proxy and credentials give one error", func(t *testing.T) {
		// Arrange
		req := newCredentialRequest(t)
		req.Flags.Runtime = sandbox.RuntimeMicrosandbox
		req.Flags.Proxy.MonitorFlag = true

		// Act
		err := checkCapabilities(req, sandbox.RuntimeDocker, sandbox.Features{})

		// Assert - only the opt-out hint, not the monitor mode one
		assert.EqualError(t, err, "--runtime microsandbox does not support the egress proxy (--proxy-monitor) or proxy credentials ([[run.proxy.credentials]] in "+req.Project.Layers[0].Path+"); pass --no-proxy to run without the proxy, or use --runtime docker")
	})

	t.Run("a runtime with every feature passes", func(t *testing.T) {
		// Arrange
		req := newCredentialRequest(t)
		req.Flags.Proxy.ProxyFlag = true
		req.Flags.Dind.DindFlag = true

		// Act
		err := checkCapabilities(req, sandbox.RuntimeDocker, sandbox.Features{Proxy: true, ProxyMonitor: true, Dind: true, CredentialInjection: true, PidsLimit: true})

		// Assert
		assert.NoError(t, err)
	})

	dindLimits := Flags{Dind: precedence.DindInput{DindFlag: true}, DindLimits: sandbox.ResourceLimits{Memory: "8g"}}
	for _, tc := range []struct {
		name     string
		req      Request
		offered  sandbox.Features
		wantWarn string
	}{
		{
			name: "an enforcing proxy passes silently without monitor mode",
			req:  msbRequest(Flags{Proxy: precedence.ProxyInput{ProxyFlag: true}}, nil),
		},
		{
			name: "a missing pids limit is silent",
			req:  msbRequest(Flags{}, nil),
		},
		{
			name: "missing path rules only warn, naming the layer",
			req: msbRequest(Flags{Proxy: precedence.ProxyInput{ProxyFlag: true}}, func(rc *agenticrc.Config) {
				rc.Run.Proxy.AllowedPaths = []agenticrc.AllowedPath{{Host: "raw.example.test", Paths: []string{"/a"}}}
			}),
			wantWarn: "--runtime microsandbox can't limit hosts to paths, so it allows the hosts of [[run.proxy.allowed_paths]] in /example.test/project/.agenticrc.toml on every path; use --runtime docker to limit them",
		},
		{
			name: "missing proxy integrations only warn, naming the layer",
			req: msbRequest(Flags{Proxy: precedence.ProxyInput{ProxyFlag: true}}, func(rc *agenticrc.Config) {
				rc.Run.Proxy.Integrations = []string{"maven"}
			}),
			wantWarn: "--runtime microsandbox ignores [run.proxy] integrations in /example.test/project/.agenticrc.toml; it can't route those tools through the proxy",
		},
		{
			name:     "missing proxy integrations from the flag warn, naming it",
			req:      msbRequest(Flags{Proxy: precedence.ProxyInput{ProxyFlag: true, Integrations: []string{"maven"}}}, nil),
			wantWarn: "--runtime microsandbox ignores --proxy-integration; it can't route those tools through the proxy",
		},
		{
			name:     "a missing docker daemon only warns, and not also about its limits",
			req:      msbRequest(dindLimits, nil),
			wantWarn: "--runtime microsandbox has no Docker daemon, so it ignores --dind",
		},
		{
			name:     "missing dind limits only warn",
			req:      msbRequest(dindLimits, nil),
			offered:  sandbox.Features{Dind: true},
			wantWarn: "--runtime microsandbox ignores the Docker daemon limits ([run.dind] or --dind-*); dockerd shares the VM's CPUs and memory",
		},
		{
			name:     "a set pids limit only warns",
			req:      msbRequest(Flags{Limits: sandbox.ResourceLimits{PidsLimit: "50"}}, nil),
			wantWarn: "--runtime microsandbox ignores the process limit ([run] pids_limit or --pids-limit); the VM has its own kernel",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			errLog := iostreamstest.StubErr(t)
			offered := msb
			offered.Dind = tc.offered.Dind

			// Act
			err := checkCapabilities(tc.req, sandbox.RuntimeDocker, offered)

			// Assert
			require.NoError(t, err)
			want := ""
			if tc.wantWarn != "" {
				want = "agentic: warning: " + tc.wantWarn + "\n"
			}
			assert.Equal(t, want, errLog.String())
		})
	}
}

func Test_proxySource(t *testing.T) {
	enabled := true
	monitorLayer := agenticrc.Layer{Path: "/example.test/outer/.agenticrc.toml", Config: &agenticrc.Config{}}
	monitorLayer.Config.Run.Proxy.Mode = agenticrc.ModeMonitor
	enabledLayer := agenticrc.Layer{Path: "/example.test/inner/.agenticrc.toml", Config: &agenticrc.Config{}}
	enabledLayer.Config.Run.Proxy.Enabled = &enabled

	for _, tc := range []struct {
		name   string
		flags  precedence.ProxyInput
		rc     agenticrc.Config
		layers []agenticrc.Layer
		want   string
	}{
		{
			name:  "names the monitor flag over the proxy flag",
			flags: precedence.ProxyInput{ProxyFlag: true, MonitorFlag: true},
			want:  "--proxy-monitor",
		},
		{
			name:   "names the monitor mode and its layer",
			rc:     agenticrc.Config{Run: agenticrc.Run{Proxy: agenticrc.Proxy{Mode: agenticrc.ModeMonitor, Enabled: &enabled}}},
			layers: []agenticrc.Layer{monitorLayer, enabledLayer},
			want:   "[run.proxy] mode in /example.test/outer/.agenticrc.toml",
		},
		{
			name:   "names enabled and its layer",
			rc:     agenticrc.Config{Run: agenticrc.Run{Proxy: agenticrc.Proxy{Enabled: &enabled}}},
			layers: []agenticrc.Layer{enabledLayer},
			want:   "[run.proxy] enabled in /example.test/inner/.agenticrc.toml",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			req := Request{Project: agenticrc.Project{Config: &tc.rc, Layers: tc.layers}, Flags: Flags{Proxy: tc.flags}}

			// Act
			source := proxySource(req)

			// Assert
			assert.Equal(t, tc.want, source)
		})
	}
}

func Test_dindSource(t *testing.T) {
	// Arrange
	enabled := true
	layer := agenticrc.Layer{Path: "/example.test/.agenticrc.toml", Config: &agenticrc.Config{}}
	layer.Config.Run.Dind.Enabled = &enabled
	req := Request{Project: agenticrc.Project{Layers: []agenticrc.Layer{layer}}}

	// Act
	source := dindSource(req)

	// Assert
	assert.Equal(t, "[run.dind] enabled in /example.test/.agenticrc.toml", source)
}

func Test_runtimeSource(t *testing.T) {
	// Arrange
	rc := &agenticrc.Config{Run: agenticrc.Run{Runtime: "microsandbox"}}
	req := Request{Project: agenticrc.Project{Config: rc, Layers: []agenticrc.Layer{{Path: "/example.test/.agenticrc.toml", Config: rc}}}}

	// Act
	source := runtimeSource(req)

	// Assert
	assert.Equal(t, `[run] runtime = "microsandbox" in /example.test/.agenticrc.toml`, source)
}

func Test_layerSource(t *testing.T) {
	hasCustom := func(rc *agenticrc.Config) bool { return rc.Run.Instructions.Custom != "" }
	outer := agenticrc.Layer{Path: "/example.test/.agenticrc.toml", Config: &agenticrc.Config{}}
	outer.Config.Run.Instructions.Custom = "outer"
	inner := agenticrc.Layer{Path: "/example.test/project/.agenticrc.toml", Config: &agenticrc.Config{}}
	inner.Config.Run.Instructions.Custom = "inner"

	t.Run("the innermost layer setting the key wins", func(t *testing.T) {
		// Act
		source := layerSource([]agenticrc.Layer{outer, inner}, "custom", hasCustom)

		// Assert
		assert.Equal(t, "custom in /example.test/project/.agenticrc.toml", source)
	})

	t.Run("skips layers that don't set the key", func(t *testing.T) {
		// Arrange
		unset := agenticrc.Layer{Path: "/example.test/project/sub/.agenticrc.toml", Config: &agenticrc.Config{}}

		// Act
		source := layerSource([]agenticrc.Layer{outer, unset}, "custom", hasCustom)

		// Assert
		assert.Equal(t, "custom in /example.test/.agenticrc.toml", source)
	})

	t.Run("falls back to the key alone", func(t *testing.T) {
		// Act
		source := layerSource(nil, "custom", hasCustom)

		// Assert
		assert.Equal(t, "custom", source)
	})
}

func Test_specFeatures(t *testing.T) {
	paths := []sandbox.AllowRule{{Host: "raw.example.test", Paths: []string{"/a"}}}

	for _, tc := range []struct {
		name string
		rs   sandbox.RunSpec
		want sandbox.Features
	}{
		{
			name: "a bare spec relies on nothing",
		},
		{
			name: "the proxy and credentials each count",
			rs: sandbox.RunSpec{
				Egress:      sandbox.EgressPolicy{Mode: sandbox.EgressEnforce},
				Credentials: []sandbox.Credential{{Name: "API_TOKEN"}},
			},
			want: sandbox.Features{Proxy: true, CredentialInjection: true},
		},
		{
			name: "path rules without the proxy don't count",
			rs:   sandbox.RunSpec{Egress: sandbox.EgressPolicy{Rules: paths}},
		},
		{
			name: "path rules count with the proxy",
			rs:   sandbox.RunSpec{Egress: sandbox.EgressPolicy{Mode: sandbox.EgressEnforce, Rules: paths}},
			want: sandbox.Features{Proxy: true, PathRules: true},
		},
		{
			name: "ports without the proxy don't count",
			rs:   sandbox.RunSpec{Ports: []sandbox.PortMapping{{Host: 3000, Guest: 3000}}},
		},
		{
			name: "ports count with the proxy",
			rs: sandbox.RunSpec{
				Egress: sandbox.EgressPolicy{Mode: sandbox.EgressEnforce},
				Ports:  []sandbox.PortMapping{{Host: 3000, Guest: 3000}},
			},
			want: sandbox.Features{Proxy: true, PortsWithProxy: true},
		},
		{
			name: "monitor mode counts too",
			rs:   sandbox.RunSpec{Egress: sandbox.EgressPolicy{Mode: sandbox.EgressMonitor}},
			want: sandbox.Features{Proxy: true, ProxyMonitor: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			need := specFeatures(tc.rs)

			// Assert
			assert.Equal(t, tc.want, need)
		})
	}
}

func Test_checkSpec(t *testing.T) {
	t.Run("passes a spec the runtime can enforce", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{Credentials: []sandbox.Credential{{Name: "API_TOKEN"}}}

		// Act
		err := checkSpec(rs, sandbox.Features{CredentialInjection: true})

		// Assert
		assert.NoError(t, err)
	})

	t.Run("refuses every missing feature at once", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{Egress: sandbox.EgressPolicy{Mode: sandbox.EgressMonitor}, Credentials: []sandbox.Credential{{Name: "API_TOKEN"}}}

		// Act
		err := checkSpec(rs, sandbox.Features{})

		// Assert
		assert.EqualError(t, err, "the runtime can't enforce the egress proxy or credential injection that the run asks for")
	})

	t.Run("refuses ports with the proxy", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{
			Egress: sandbox.EgressPolicy{Mode: sandbox.EgressEnforce},
			Ports:  []sandbox.PortMapping{{Host: 3000, Guest: 3000}},
		}

		// Act
		err := checkSpec(rs, sandbox.Features{Proxy: true})

		// Assert
		assert.EqualError(t, err, "the runtime can't enforce published ports with the egress proxy that the run asks for")
	})

	t.Run("refuses monitor mode on a runtime that only enforces", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{Egress: sandbox.EgressPolicy{Mode: sandbox.EgressMonitor}}

		// Act
		err := checkSpec(rs, sandbox.Features{Proxy: true})

		// Assert
		assert.EqualError(t, err, "the runtime can't enforce proxy monitor mode that the run asks for")
	})
}
