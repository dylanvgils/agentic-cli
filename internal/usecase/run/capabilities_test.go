package run

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_requiredFeatures(t *testing.T) {
	enabled := true
	proxyFlag := resolve.ProxyInput{ProxyFlag: true}
	paths := []config.RCAllowedPath{{Host: "raw.example.test", Paths: []string{"/a"}}}
	creds := []config.RCCredential{{Preset: "github", Secret: "/example.test/token"}}

	for _, tc := range []struct {
		name  string
		flags Flags
		rc    config.AgenticRC
		want  sandbox.Features
	}{
		{
			name: "a bare run needs only the pids limit",
			want: sandbox.Features{PidsLimit: true},
		},
		{
			name:  "flags ask for the proxy and dind",
			flags: Flags{Proxy: resolve.ProxyInput{MonitorFlag: true}, Dind: resolve.DindInput{DindFlag: true}},
			want:  sandbox.Features{Proxy: true, ProxyMonitor: true, Dind: true, PidsLimit: true},
		},
		{
			name: "config asks for the proxy and dind",
			rc:   config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &enabled}, Dind: config.RCDind{Enabled: &enabled}}},
			want: sandbox.Features{Proxy: true, Dind: true, PidsLimit: true},
		},
		{
			name: "off switches drop the need",
			flags: Flags{
				Proxy: resolve.ProxyInput{NoProxy: true},
				Dind:  resolve.DindInput{NoDindFlag: true},
			},
			rc:   config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &enabled}, Dind: config.RCDind{Enabled: &enabled}}},
			want: sandbox.Features{PidsLimit: true},
		},
		{
			name:  "dind limits ask for limit support with dind",
			flags: Flags{Dind: resolve.DindInput{DindFlag: true}, DindLimits: sandbox.ResourceLimits{Memory: "8g"}},
			want:  sandbox.Features{Dind: true, DindLimits: true, PidsLimit: true},
		},
		{
			name: "dind limits without dind have nothing to limit",
			rc:   config.AgenticRC{Run: config.RCRun{Dind: config.RCDind{RCLimits: config.RCLimits{Memory: "8g"}}}},
			want: sandbox.Features{PidsLimit: true},
		},
		{
			name: "credentials alone don't ask for the proxy",
			rc:   config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Credentials: creds}}},
			want: sandbox.Features{CredentialInjection: true, PidsLimit: true},
		},
		{
			name: "path rules without the proxy or credentials need nothing",
			rc:   config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{AllowedPaths: paths}}},
			want: sandbox.Features{PidsLimit: true},
		},
		{
			name:  "path rules ask for path rule support with the proxy",
			flags: Flags{Proxy: proxyFlag},
			rc:    config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{AllowedPaths: paths}}},
			want:  sandbox.Features{Proxy: true, PathRules: true, PidsLimit: true},
		},
		{
			name: "path rules ask for path rule support with credentials",
			rc:   config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{AllowedPaths: paths, Credentials: creds}}},
			want: sandbox.Features{CredentialInjection: true, PathRules: true, PidsLimit: true},
		},
		{
			name: "proxy integrations without the proxy or credentials need nothing",
			rc:   config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Integrations: []string{"maven"}}}},
			want: sandbox.Features{PidsLimit: true},
		},
		{
			name:  "proxy integrations ask for integration support with the proxy",
			flags: Flags{Proxy: proxyFlag},
			rc:    config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Integrations: []string{"maven"}}}},
			want:  sandbox.Features{Proxy: true, ProxyIntegrations: true, PidsLimit: true},
		},
		{
			name: "proxy integrations ask for integration support with credentials",
			rc:   config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Integrations: []string{"maven"}, Credentials: creds}}},
			want: sandbox.Features{CredentialInjection: true, ProxyIntegrations: true, PidsLimit: true},
		},
		{
			name:  "the integration flag asks for integration support with the proxy",
			flags: Flags{Proxy: resolve.ProxyInput{ProxyFlag: true, Integrations: []string{"maven"}}},
			want:  sandbox.Features{Proxy: true, ProxyIntegrations: true, PidsLimit: true},
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
			req:     msbRequest(Flags{Proxy: resolve.ProxyInput{ProxyFlag: true}}, nil),
			offered: noProxy,
			wantErr: "--runtime microsandbox does not support the egress proxy (--proxy); pass --no-proxy to run without the proxy, or use --runtime docker",
		},
		{
			name:    "a missing proxy asked for in config names the layer",
			req:     msbRequest(Flags{}, func(rc *config.AgenticRC) { enabled := true; rc.Run.Proxy.Enabled = &enabled }),
			offered: noProxy,
			wantErr: "--runtime microsandbox does not support the egress proxy ([run.proxy] enabled in /example.test/project/.agenticrc.toml); pass --no-proxy to run without the proxy, or use --runtime docker",
		},
		{
			name:    "a missing monitor mode is an error pointing to --proxy",
			req:     msbRequest(Flags{Proxy: resolve.ProxyInput{MonitorFlag: true}}, nil),
			offered: msb,
			wantErr: "--runtime microsandbox does not support proxy monitor mode (--proxy-monitor); pass --proxy to enforce the allowlist instead, or use --runtime docker",
		},
		{
			name:    "a missing monitor mode asked for in config names the layer",
			req:     msbRequest(Flags{}, func(rc *config.AgenticRC) { rc.Run.Proxy.Mode = config.ModeMonitor }),
			offered: msb,
			wantErr: "--runtime microsandbox does not support proxy monitor mode ([run.proxy] mode in /example.test/project/.agenticrc.toml); pass --proxy to enforce the allowlist instead, or use --runtime docker",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			err := checkCapabilities(tc.req, tc.offered)

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
		err := checkCapabilities(req, sandbox.Features{})

		// Assert - only the opt-out hint, not the monitor mode one
		assert.EqualError(t, err, "--runtime microsandbox does not support the egress proxy (--proxy-monitor) or proxy credentials ([[run.proxy.credentials]] in "+req.Project.Layers[0].Path+"); pass --no-proxy to run without the proxy, or use --runtime docker")
	})

	t.Run("a runtime with every feature passes", func(t *testing.T) {
		// Arrange
		req := newCredentialRequest(t)
		req.Flags.Proxy.ProxyFlag = true
		req.Flags.Dind.DindFlag = true

		// Act
		err := checkCapabilities(req, sandbox.Features{Proxy: true, ProxyMonitor: true, Dind: true, CredentialInjection: true, PidsLimit: true})

		// Assert
		assert.NoError(t, err)
	})

	dindLimits := Flags{Dind: resolve.DindInput{DindFlag: true}, DindLimits: sandbox.ResourceLimits{Memory: "8g"}}
	for _, tc := range []struct {
		name     string
		req      Request
		offered  sandbox.Features
		wantWarn string
	}{
		{
			name: "an enforcing proxy passes silently without monitor mode",
			req:  msbRequest(Flags{Proxy: resolve.ProxyInput{ProxyFlag: true}}, nil),
		},
		{
			name: "a missing pids limit is silent",
			req:  msbRequest(Flags{}, nil),
		},
		{
			name: "missing path rules only warn, naming the layer",
			req: msbRequest(Flags{Proxy: resolve.ProxyInput{ProxyFlag: true}}, func(rc *config.AgenticRC) {
				rc.Run.Proxy.AllowedPaths = []config.RCAllowedPath{{Host: "raw.example.test", Paths: []string{"/a"}}}
			}),
			wantWarn: "--runtime microsandbox can't limit hosts to paths, so it allows the hosts of [[run.proxy.allowed_paths]] in /example.test/project/.agenticrc.toml on every path; use --runtime docker to limit them",
		},
		{
			name: "missing proxy integrations only warn, naming the layer",
			req: msbRequest(Flags{Proxy: resolve.ProxyInput{ProxyFlag: true}}, func(rc *config.AgenticRC) {
				rc.Run.Proxy.Integrations = []string{"maven"}
			}),
			wantWarn: "--runtime microsandbox ignores [run.proxy] integrations in /example.test/project/.agenticrc.toml; it can't route those tools through the proxy",
		},
		{
			name:     "missing proxy integrations from the flag warn, naming it",
			req:      msbRequest(Flags{Proxy: resolve.ProxyInput{ProxyFlag: true, Integrations: []string{"maven"}}}, nil),
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			errLog := iostreamstest.StubErr(t)
			offered := msb
			offered.Dind = tc.offered.Dind

			// Act
			err := checkCapabilities(tc.req, offered)

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
	monitorLayer := config.RCLayer{Path: "/example.test/outer/.agenticrc.toml", RC: &config.AgenticRC{}}
	monitorLayer.RC.Run.Proxy.Mode = config.ModeMonitor
	enabledLayer := config.RCLayer{Path: "/example.test/inner/.agenticrc.toml", RC: &config.AgenticRC{}}
	enabledLayer.RC.Run.Proxy.Enabled = &enabled

	for _, tc := range []struct {
		name   string
		flags  resolve.ProxyInput
		rc     config.AgenticRC
		layers []config.RCLayer
		want   string
	}{
		{
			name:  "names the flag",
			flags: resolve.ProxyInput{ProxyFlag: true},
			want:  "--proxy",
		},
		{
			name:  "names the monitor flag over the proxy flag",
			flags: resolve.ProxyInput{ProxyFlag: true, MonitorFlag: true},
			want:  "--proxy-monitor",
		},
		{
			name:   "names the monitor mode and its layer",
			rc:     config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Mode: config.ModeMonitor, Enabled: &enabled}}},
			layers: []config.RCLayer{monitorLayer, enabledLayer},
			want:   "[run.proxy] mode in /example.test/outer/.agenticrc.toml",
		},
		{
			name:   "names enabled and its layer",
			rc:     config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &enabled}}},
			layers: []config.RCLayer{enabledLayer},
			want:   "[run.proxy] enabled in /example.test/inner/.agenticrc.toml",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			req := Request{Project: config.Project{RC: &tc.rc, Layers: tc.layers}, Flags: Flags{Proxy: tc.flags}}

			// Act
			source := proxySource(req)

			// Assert
			assert.Equal(t, tc.want, source)
		})
	}
}

func Test_dindSource(t *testing.T) {
	enabled := true

	t.Run("names the flag", func(t *testing.T) {
		// Arrange
		req := Request{}
		req.Flags.Dind.DindFlag = true

		// Act
		source := dindSource(req)

		// Assert
		assert.Equal(t, "--dind", source)
	})

	t.Run("names enabled and its layer", func(t *testing.T) {
		// Arrange
		layer := config.RCLayer{Path: "/example.test/.agenticrc.toml", RC: &config.AgenticRC{}}
		layer.RC.Run.Dind.Enabled = &enabled
		req := Request{Project: config.Project{Layers: []config.RCLayer{layer}}}

		// Act
		source := dindSource(req)

		// Assert
		assert.Equal(t, "[run.dind] enabled in /example.test/.agenticrc.toml", source)
	})
}

func Test_runtimeSource(t *testing.T) {
	t.Run("names the flag", func(t *testing.T) {
		// Arrange
		req := runtimeRequest(sandbox.RuntimeMicrosandbox)

		// Act
		source := runtimeSource(req)

		// Assert
		assert.Equal(t, "--runtime microsandbox", source)
	})

	t.Run("names the key and its layer", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Runtime: "microsandbox"}}
		req := Request{Project: config.Project{RC: rc, Layers: []config.RCLayer{{Path: "/example.test/.agenticrc.toml", RC: rc}}}}

		// Act
		source := runtimeSource(req)

		// Assert
		assert.Equal(t, `[run] runtime = "microsandbox" in /example.test/.agenticrc.toml`, source)
	})
}

func Test_credentialsSource(t *testing.T) {
	// Arrange
	layer := config.RCLayer{Path: "/example.test/.agenticrc.toml", RC: &config.AgenticRC{}}
	layer.RC.Run.Proxy.Credentials = []config.RCCredential{{Hosts: []string{"api.example.test"}}}
	req := Request{Project: config.Project{Layers: []config.RCLayer{layer}}}

	// Act
	source := credentialsSource(req)

	// Assert
	assert.Equal(t, "[[run.proxy.credentials]] in /example.test/.agenticrc.toml", source)
}

func Test_pathRulesSource(t *testing.T) {
	// Arrange
	layer := config.RCLayer{Path: "/example.test/.agenticrc.toml", RC: &config.AgenticRC{}}
	layer.RC.Run.Proxy.AllowedPaths = []config.RCAllowedPath{{Host: "raw.example.test", Paths: []string{"/a"}}}
	req := Request{Project: config.Project{Layers: []config.RCLayer{layer}}}

	// Act
	source := pathRulesSource(req)

	// Assert
	assert.Equal(t, "[[run.proxy.allowed_paths]] in /example.test/.agenticrc.toml", source)
}

func Test_layerSource(t *testing.T) {
	hasCustom := func(rc *config.AgenticRC) bool { return rc.Run.Instructions.Custom != "" }
	outer := config.RCLayer{Path: "/example.test/.agenticrc.toml", RC: &config.AgenticRC{}}
	outer.RC.Run.Instructions.Custom = "outer"
	inner := config.RCLayer{Path: "/example.test/project/.agenticrc.toml", RC: &config.AgenticRC{}}
	inner.RC.Run.Instructions.Custom = "inner"

	t.Run("the innermost layer setting the key wins", func(t *testing.T) {
		// Act
		source := layerSource([]config.RCLayer{outer, inner}, "custom", hasCustom)

		// Assert
		assert.Equal(t, "custom in /example.test/project/.agenticrc.toml", source)
	})

	t.Run("skips layers that don't set the key", func(t *testing.T) {
		// Arrange
		unset := config.RCLayer{Path: "/example.test/project/sub/.agenticrc.toml", RC: &config.AgenticRC{}}

		// Act
		source := layerSource([]config.RCLayer{outer, unset}, "custom", hasCustom)

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

	t.Run("refuses monitor mode on a runtime that only enforces", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{Egress: sandbox.EgressPolicy{Mode: sandbox.EgressMonitor}}

		// Act
		err := checkSpec(rs, sandbox.Features{Proxy: true})

		// Assert
		assert.EqualError(t, err, "the runtime can't enforce proxy monitor mode that the run asks for")
	})
}
