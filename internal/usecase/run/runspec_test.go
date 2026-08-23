package run

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuild(t *testing.T) {
	stubEnsureNamedVolumes(t, func([]string, string, string, string) error { return nil })
	stubEnsureNetwork(t, func() error { return nil })

	t.Run("volumes wired", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), Volumes: []string{"/host:/container"}}

		// Act
		rs, err := Build(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, rs.Volumes, "/host:/container")
	})

	t.Run("instructions mount wired after the tool's base mounts", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), InstructionsMount: "/tmp/snapshot.md:$CONTAINER_HOME/.claude/CLAUDE.md"}

		// Act
		rs, err := Build(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		require.Contains(t, rs.Volumes, "/tmp/snapshot.md:$CONTAINER_HOME/.claude/CLAUDE.md")
		baseMountIdx := slices.Index(rs.Volumes, tools.Configs["claude"].Runtime.Mounts()[1])
		instructionsMountIdx := slices.Index(rs.Volumes, "/tmp/snapshot.md:$CONTAINER_HOME/.claude/CLAUDE.md")
		assert.Less(t, baseMountIdx, instructionsMountIdx, "instructions mount must overlay the base directory mount, so it must be listed after it")
	})

	t.Run("read-only mounts wired after every other volume", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{
			ToolHome:          t.TempDir(),
			InstructionsMount: "/tmp/snapshot.md:$CONTAINER_HOME/.claude/CLAUDE.md",
			ReadOnlyMounts:    []string{"$PWD/flagsecret:/workspace/flagsecret"},
		}
		rc := &config.AgenticRC{Run: config.RCRun{ReadOnlyMounts: []string{"$PWD/secret:/workspace/secret"}}}

		// Act
		rs, err := Build(target, in, tools.Configs["claude"], rc)

		// Assert
		require.NoError(t, err)
		require.Contains(t, rs.Volumes, "$PWD/secret:/workspace/secret:ro")
		require.Contains(t, rs.Volumes, "$PWD/flagsecret:/workspace/flagsecret:ro")
		roIdx := slices.Index(rs.Volumes, "$PWD/secret:/workspace/secret:ro")
		flagRoIdx := slices.Index(rs.Volumes, "$PWD/flagsecret:/workspace/flagsecret:ro")
		instructionsIdx := slices.Index(rs.Volumes, "/tmp/snapshot.md:$CONTAINER_HOME/.claude/CLAUDE.md")
		assert.Greater(t, roIdx, instructionsIdx, "read-only sub-path mounts must be last so they shadow every other mount")
		assert.Greater(t, flagRoIdx, instructionsIdx, "flag-supplied read-only mounts must be last too")
	})

	t.Run("instructions mount omitted when empty", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}

		// Act
		rs, err := Build(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		for _, v := range rs.Volumes {
			assert.NotContains(t, v, "CLAUDE.md")
		}
	})

	t.Run("secrets wired", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), Secrets: []string{"mytoken:/tmp/token"}}

		// Act
		rs, err := Build(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"mytoken:/tmp/token"}, rs.Secrets)
	})

	t.Run("resource limits wired", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}
		rc := &config.AgenticRC{Run: config.RCRun{PidsLimit: "512", CPUs: "2", Memory: "2g"}}

		// Act
		rs, err := Build(target, in, tools.Configs["claude"], rc)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "512", rs.PidsLimit)
		assert.Equal(t, "2", rs.CPUs)
		assert.Equal(t, "2g", rs.Memory)
	})

	t.Run("dry run wired", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), DryRun: true}

		// Act
		rs, err := Build(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.True(t, rs.DryRun)
	})

	t.Run("tmpfs mounts wired", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}

		// Act
		rs, err := Build(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.NotEmpty(t, rs.TmpfsMounts)
	})

	t.Run("tool home wired", func(t *testing.T) {
		// Arrange
		customHome := t.TempDir()
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: customHome}

		// Act
		rs, err := Build(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, customHome, rs.ToolHome)
	})

	t.Run("skip entrypoint wired", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude", SkipEntrypoint: true}
		in := Input{ToolHome: t.TempDir()}

		// Act
		rs, err := Build(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.True(t, rs.SkipEntrypoint)
	})

	t.Run("ensure named volumes error propagates", func(t *testing.T) {
		// Arrange
		stubEnsureNamedVolumes(t, func([]string, string, string, string) error { return fmt.Errorf("volume error") })
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}

		// Act
		_, err := Build(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "volume error")
	})

	t.Run("ensure network error propagates", func(t *testing.T) {
		// Arrange
		stubEnsureNetwork(t, func() error { return fmt.Errorf("network error") })
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}

		// Act
		_, err := Build(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "network error")
	})

	t.Run("proxy wired with merged allowlist and image", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), ProxyEnabled: true}
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{AllowedHosts: []string{"extra.example.com"}}}}

		// Act
		rs, err := Build(target, in, tools.Configs["claude"], rc)

		// Assert
		require.NoError(t, err)
		assert.True(t, rs.ProxyEnabled)
		assert.Equal(t, tools.ProxyImage, rs.ProxyImage)
		assert.Equal(t, []string{".anthropic.com", ".claude.ai", ".claude.com", "extra.example.com"}, rs.ProxyAllow)
		assert.NotEmpty(t, rs.ProxyLogDir)
	})

	t.Run("proxy off leaves spec unproxied", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}

		// Act
		rs, err := Build(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.False(t, rs.ProxyEnabled)
		assert.Empty(t, rs.ProxyLogDir)
	})

	t.Run("proxy enabled skips agentic-net check since startProxy ensures it itself", func(t *testing.T) {
		// Arrange
		stubEnsureNetwork(t, func() error { return fmt.Errorf("network error") })
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), ProxyEnabled: true}

		// Act
		rs, err := Build(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.True(t, rs.ProxyEnabled)
	})

	t.Run("proxy monitor wired", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), ProxyEnabled: true, ProxyMonitor: true}

		// Act
		rs, err := Build(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.True(t, rs.ProxyEnabled)
		assert.True(t, rs.ProxyMonitor)
	})

	t.Run("audit wired with watch paths and log dir", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		home := t.TempDir()
		in := Input{ToolHome: home, AuditEnabled: true}
		rc := &config.AgenticRC{Run: config.RCRun{Audit: config.RCAudit{Exclude: []string{"target"}}}}

		// Act
		rs, err := Build(target, in, tools.Configs["claude"], rc)

		// Assert
		require.NoError(t, err)
		assert.True(t, rs.AuditEnabled)
		assert.Contains(t, rs.AuditPaths, filepath.Join(home, "claude", "data"))
		assert.Equal(t, []string{"target"}, rs.AuditExclude)
		assert.NotEmpty(t, rs.AuditLogDir)
	})

	t.Run("audit off leaves spec unaudited", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}

		// Act
		rs, err := Build(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.False(t, rs.AuditEnabled)
		assert.Empty(t, rs.AuditLogDir)
	})

	t.Run("marketplace names wired into AGENTIC_MARKETPLACES env", func(t *testing.T) {
		// Arrange
		stubSyncMarketplaces(t, func(entries []marketplace.Entry, dirFor func(marketplace.Entry) string) ([]marketplace.Result, error) {
			return []marketplace.Result{{Entry: entries[0], Dir: dirFor(entries[0])}}, nil
		})
		stubRecordMarketplaceUsage(t, func(string, []marketplace.Result, string) error { return nil })
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}
		rc := &config.AgenticRC{Marketplaces: []config.RCMarketplace{{Name: "acme", URL: "git@example.com:acme.git"}}}

		// Act
		rs, err := Build(target, in, tools.Configs["claude"], rc)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, rs.Env, "AGENTIC_MARKETPLACES=acme")
	})

	t.Run("no marketplaces configured leaves AGENTIC_MARKETPLACES unset", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}

		// Act
		rs, err := Build(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		for _, e := range rs.Env {
			assert.NotContains(t, e, "AGENTIC_MARKETPLACES")
		}
	})
}

func TestBuildWithInstructions(t *testing.T) {
	stubEnsureNamedVolumes(t, func([]string, string, string, string) error { return nil })
	stubEnsureNetwork(t, func() error { return nil })

	// BuildInstructions' own content rules (sections, formatting) are covered by
	// TestBuildInstructions, and PrepareInstructions' merge/sync-back behavior is
	// covered by TestPrepareInstructions - these subtests only cover what
	// BuildWithInstructions itself adds: threading content -> snapshot -> mount
	// into Build, and calling cleanup on a Build failure instead of leaking.

	t.Run("wires the generated content into the RunSpec's instructions mount", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}
		require.NoError(t, tools.Configs["claude"].Runtime.Setup(in.ToolHome))

		// Act
		rs, cleanup, err := BuildWithInstructions(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		t.Cleanup(cleanup)
		instructionsVolume := findVolumeSuffix(t, rs.Volumes, ":$CONTAINER_HOME/.claude/CLAUDE.md")
		got, err := os.ReadFile(mount.HostPart(instructionsVolume))
		require.NoError(t, err)
		assert.Contains(t, string(got), "# Agentic container environment")
	})

	t.Run("returned cleanup is wired to a real finalize, not a no-op", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}
		require.NoError(t, tools.Configs["claude"].Runtime.Setup(in.ToolHome))
		rs, cleanup, err := BuildWithInstructions(target, in, tools.Configs["claude"], &config.AgenticRC{})
		require.NoError(t, err)
		instructionsVolume := findVolumeSuffix(t, rs.Volumes, ":$CONTAINER_HOME/.claude/CLAUDE.md")
		snapshotPath := mount.HostPart(instructionsVolume)

		// Act
		cleanup()

		// Assert
		_, statErr := os.Stat(snapshotPath)
		assert.True(t, os.IsNotExist(statErr))
	})

	t.Run("build error finalizes and removes the snapshot immediately, rather than leaking it", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), Env: []string{"TOOL_HOME=nope"}}
		require.NoError(t, tools.Configs["claude"].Runtime.Setup(in.ToolHome))
		before, err := filepath.Glob(filepath.Join(os.TempDir(), "agentic-instructions-*"))
		require.NoError(t, err)

		// Act
		_, cleanup, err := BuildWithInstructions(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.Error(t, err)
		assert.NotPanics(t, func() { cleanup() })
		after, globErr := filepath.Glob(filepath.Join(os.TempDir(), "agentic-instructions-*"))
		require.NoError(t, globErr)
		assert.Len(t, after, len(before), "snapshot temp file should already be cleaned up when Build fails")
	})
}

func TestToolNeedsMarketplaceSync(t *testing.T) {
	t.Run("tool without marketplace support returns false", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Marketplaces: []config.RCMarketplace{{Name: "acme", URL: "git@example.com:acme.git"}}}

		// Act
		needs := ToolNeedsMarketplaceSync(tools.Configs["opencode"], rc, "opencode")

		// Assert
		assert.False(t, needs)
	})

	t.Run("marketplace-capable tool with no marketplaces configured returns false", func(t *testing.T) {
		// Act
		needs := ToolNeedsMarketplaceSync(tools.Configs["claude"], &config.AgenticRC{}, "claude")

		// Assert
		assert.False(t, needs)
	})

	t.Run("marketplace-capable tool with marketplaces configured returns true", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Marketplaces: []config.RCMarketplace{{Name: "acme", URL: "git@example.com:acme.git"}}}

		// Act
		needs := ToolNeedsMarketplaceSync(tools.Configs["claude"], rc, "claude")

		// Assert
		assert.True(t, needs)
	})
}

func Test_syncToolMarketplaces(t *testing.T) {
	t.Run("tool that doesn't need marketplace sync returns nil", func(t *testing.T) {
		// Act
		mounts, names, err := syncToolMarketplaces("/home", "claude", tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Nil(t, mounts)
		assert.Nil(t, names)
	})

	t.Run("syncs configured marketplaces and returns mounts and names", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		var gotEntries []marketplace.Entry
		var gotDir string
		stubSyncMarketplaces(t, func(entries []marketplace.Entry, dirFor func(marketplace.Entry) string) ([]marketplace.Result, error) {
			gotEntries = entries
			gotDir = dirFor(entries[0])
			return []marketplace.Result{{Entry: entries[0], Dir: dirFor(entries[0])}}, nil
		})
		rc := &config.AgenticRC{Marketplaces: []config.RCMarketplace{{Name: "acme", URL: "git@example.com:acme.git"}}}

		// Act
		mounts, names, err := syncToolMarketplaces(home, "claude", tools.Configs["claude"], rc)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []marketplace.Entry{{Name: "acme", URL: "git@example.com:acme.git"}}, gotEntries)
		wantDirName := marketplace.CloneDirName("git@example.com:acme.git")
		assert.Equal(t, filepath.Join(home, "marketplaces", wantDirName), gotDir)
		assert.Equal(t, []string{tools.Configs["claude"].Runtime.MarketplaceMount("acme", "git@example.com:acme.git")}, mounts)
		assert.Equal(t, []string{"acme"}, names)
	})

	t.Run("sync error is wrapped with tool name", func(t *testing.T) {
		// Arrange
		stubSyncMarketplaces(t, func([]marketplace.Entry, func(marketplace.Entry) string) ([]marketplace.Result, error) {
			return nil, fmt.Errorf("clone failed")
		})
		rc := &config.AgenticRC{Marketplaces: []config.RCMarketplace{{Name: "acme", URL: "git@example.com:acme.git"}}}

		// Act
		_, _, err := syncToolMarketplaces(t.TempDir(), "claude", tools.Configs["claude"], rc)

		// Assert
		assert.ErrorContains(t, err, "claude")
		assert.ErrorContains(t, err, "clone failed")
	})

	t.Run("stale result still returns a mount and name", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		stubSyncMarketplaces(t, func(entries []marketplace.Entry, dirFor func(marketplace.Entry) string) ([]marketplace.Result, error) {
			return []marketplace.Result{{Entry: entries[0], Dir: dirFor(entries[0]), Stale: true, Warning: fmt.Errorf("offline")}}, nil
		})
		rc := &config.AgenticRC{Marketplaces: []config.RCMarketplace{{Name: "acme", URL: "git@example.com:acme.git"}}}

		// Act
		mounts, names, err := syncToolMarketplaces(home, "claude", tools.Configs["claude"], rc)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{tools.Configs["claude"].Runtime.MarketplaceMount("acme", "git@example.com:acme.git")}, mounts)
		assert.Equal(t, []string{"acme"}, names)
	})

	t.Run("records usage against the sync results after a successful sync", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		wantResults := []marketplace.Result{{Entry: marketplace.Entry{Name: "acme", URL: "git@example.com:acme.git"}, Dir: filepath.Join(home, "marketplaces", marketplace.CloneDirName("git@example.com:acme.git"))}}
		stubSyncMarketplaces(t, func(entries []marketplace.Entry, dirFor func(marketplace.Entry) string) ([]marketplace.Result, error) {
			return wantResults, nil
		})
		var gotBaseDir string
		var gotResults []marketplace.Result
		stubRecordMarketplaceUsage(t, func(baseDir string, results []marketplace.Result, _ string) error {
			gotBaseDir = baseDir
			gotResults = results
			return nil
		})
		rc := &config.AgenticRC{Marketplaces: []config.RCMarketplace{{Name: "acme", URL: "git@example.com:acme.git"}}}

		// Act
		_, _, err := syncToolMarketplaces(home, "claude", tools.Configs["claude"], rc)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(home, "marketplaces"), gotBaseDir)
		assert.Equal(t, wantResults, gotResults)
	})

	t.Run("usage recording failure does not fail the run", func(t *testing.T) {
		// Arrange
		stubSyncMarketplaces(t, func(entries []marketplace.Entry, dirFor func(marketplace.Entry) string) ([]marketplace.Result, error) {
			return []marketplace.Result{{Entry: entries[0], Dir: dirFor(entries[0])}}, nil
		})
		stubRecordMarketplaceUsage(t, func(string, []marketplace.Result, string) error {
			return fmt.Errorf("disk error")
		})
		rc := &config.AgenticRC{Marketplaces: []config.RCMarketplace{{Name: "acme", URL: "git@example.com:acme.git"}}}

		// Act
		mounts, names, err := syncToolMarketplaces(t.TempDir(), "claude", tools.Configs["claude"], rc)

		// Assert
		require.NoError(t, err)
		assert.Len(t, mounts, 1)
		assert.Equal(t, []string{"acme"}, names)
	})
}

func Test_collectVolumes(t *testing.T) {
	t.Run("ordering", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_EXTRA_MOUNTS", "envvol:/mnt/env")
		rc := &config.AgenticRC{Run: config.RCRun{ExtraMounts: []string{"rcvol:/mnt/rc"}}}

		// Act
		result := collectVolumes([]string{"tool:/mnt/tool"}, []string{"flagvol:/mnt/flag"}, rc)

		// Assert
		assert.Equal(t, []string{
			"tool:/mnt/tool",
			"envvol:/mnt/env",
			"flagvol:/mnt/flag",
			"rcvol:/mnt/rc",
		}, result)
	})

	t.Run("empty env skipped", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_EXTRA_MOUNTS", "")
		rc := &config.AgenticRC{}

		// Act
		result := collectVolumes([]string{"tool:/mnt/tool"}, nil, rc)

		// Assert
		assert.Equal(t, []string{"tool:/mnt/tool"}, result)
	})

	t.Run("no sources returns empty", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_EXTRA_MOUNTS", "")
		rc := &config.AgenticRC{}

		// Act
		result := collectVolumes(nil, nil, rc)

		// Assert
		assert.Empty(t, result)
	})

	t.Run("does not mutate tool mounts", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_EXTRA_MOUNTS", "")
		toolMounts := []string{"tool:/mnt/tool"}
		rc := &config.AgenticRC{}

		// Act
		result := collectVolumes(toolMounts, []string{"extra:/mnt/extra"}, rc)

		// Assert
		assert.Len(t, toolMounts, 1, "original toolMounts slice should not be modified")
		assert.Len(t, result, 2)
	})
}

func Test_collectSecrets(t *testing.T) {
	t.Run("ordering", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_SECRETS", "envtoken:/tmp/env")
		rc := &config.AgenticRC{Run: config.RCRun{Secrets: []string{"rctoken:/tmp/rc"}}}

		// Act
		result := collectSecrets([]string{"flagtoken:/tmp/flag"}, rc)

		// Assert
		assert.Equal(t, []string{
			"envtoken:/tmp/env",
			"flagtoken:/tmp/flag",
			"rctoken:/tmp/rc",
		}, result)
	})

	t.Run("empty env skipped", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_SECRETS", "")
		rc := &config.AgenticRC{}

		// Act
		result := collectSecrets([]string{"flagtoken:/tmp/flag"}, rc)

		// Assert
		assert.Equal(t, []string{"flagtoken:/tmp/flag"}, result)
	})

	t.Run("all empty returns nil", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_SECRETS", "")
		rc := &config.AgenticRC{}

		// Act
		result := collectSecrets(nil, rc)

		// Assert
		assert.Nil(t, result)
	})
}

func Test_collectReadOnlyMounts(t *testing.T) {
	t.Run("ordering", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{ReadOnlyMounts: []string{"rcro:/mnt/rc"}}}

		// Act
		result := collectReadOnlyMounts([]string{"flagro:/mnt/flag"}, rc)

		// Assert
		assert.Equal(t, []string{
			"flagro:/mnt/flag",
			"rcro:/mnt/rc",
		}, result)
	})

	t.Run("only flags", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}

		// Act
		result := collectReadOnlyMounts([]string{"flagro:/mnt/flag"}, rc)

		// Assert
		assert.Equal(t, []string{"flagro:/mnt/flag"}, result)
	})

	t.Run("all empty returns nil", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}

		// Act
		result := collectReadOnlyMounts(nil, rc)

		// Assert
		assert.Nil(t, result)
	})
}

func Test_collectEnv(t *testing.T) {
	t.Run("ordering, flag wins over rc on duplicate key", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Env: []string{"FOO=fromrc"}}}

		// Act
		result := collectEnv([]string{"FOO=fromflag"}, rc)

		// Assert
		assert.Equal(t, []string{"FOO=fromrc", "FOO=fromflag"}, result)
	})

	t.Run("no sources returns empty", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}

		// Act
		result := collectEnv(nil, rc)

		// Assert
		assert.Empty(t, result)
	})

	t.Run("does not mutate rc env slice", func(t *testing.T) {
		// Arrange
		rcEnv := []string{"FOO=bar"}
		rc := &config.AgenticRC{Run: config.RCRun{Env: rcEnv}}

		// Act
		result := collectEnv([]string{"BAZ=qux"}, rc)

		// Assert
		assert.Len(t, rcEnv, 1, "original rc env slice should not be modified")
		assert.Len(t, result, 2)
	})
}

func Test_validateEnv(t *testing.T) {
	t.Run("accepts ordinary KEY=VALUE", func(t *testing.T) {
		// Act
		err := validateEnv([]string{"MAVEN_OPTS=-Dfoo=bar"}, false)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("accepts bare KEY", func(t *testing.T) {
		// Act
		err := validateEnv([]string{"CI"}, false)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("accepts overriding terminal vars", func(t *testing.T) {
		// Act
		err := validateEnv([]string{"NO_COLOR=1", "TERM=dumb"}, true)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("accepts proxy var when proxy disabled", func(t *testing.T) {
		// Act
		err := validateEnv([]string{"HTTP_PROXY=http://my-corp-proxy"}, false)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("rejects proxy var when proxy enabled", func(t *testing.T) {
		// Act
		err := validateEnv([]string{"HTTP_PROXY=http://evil"}, true)

		// Assert
		assert.ErrorContains(t, err, "HTTP_PROXY")
	})

	t.Run("rejects bare reserved name regardless of proxy", func(t *testing.T) {
		// Act
		err := validateEnv([]string{"TOOL_HOME"}, false)

		// Assert
		assert.ErrorContains(t, err, "TOOL_HOME")
	})
}

func Test_resolveResourceLimits(t *testing.T) {
	t.Run("rc fills empty flags", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{PidsLimit: "512", CPUs: "2", Memory: "2g"}}

		// Act
		result := resolveResourceLimits("", "", "", rc)

		// Assert
		assert.Equal(t, "512", result.pidsLimit)
		assert.Equal(t, "2", result.cpus)
		assert.Equal(t, "2g", result.memory)
	})

	t.Run("flag takes precedence over rc", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{PidsLimit: "512", CPUs: "2", Memory: "2g"}}

		// Act
		result := resolveResourceLimits("1024", "4", "4g", rc)

		// Assert
		assert.Equal(t, "1024", result.pidsLimit)
		assert.Equal(t, "4", result.cpus)
		assert.Equal(t, "4g", result.memory)
	})

	t.Run("partial flags rc fills rest", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{PidsLimit: "512", CPUs: "2", Memory: "2g"}}

		// Act
		result := resolveResourceLimits("1024", "", "", rc)

		// Assert
		assert.Equal(t, "1024", result.pidsLimit)
		assert.Equal(t, "2", result.cpus)
		assert.Equal(t, "2g", result.memory)
	})

	t.Run("falls back to env var when flag and rc unset", func(t *testing.T) {
		// Arrange
		t.Setenv(config.EnvPidsLimit, "256")
		t.Setenv(config.EnvCPUs, "1")
		t.Setenv(config.EnvMemory, "1g")

		// Act
		result := resolveResourceLimits("", "", "", &config.AgenticRC{})

		// Assert
		assert.Equal(t, "256", result.pidsLimit)
		assert.Equal(t, "1", result.cpus)
		assert.Equal(t, "1g", result.memory)
	})

	t.Run("falls back to hardcoded default when nothing else set", func(t *testing.T) {
		// Act
		result := resolveResourceLimits("", "", "", &config.AgenticRC{})

		// Assert - always resolved, never left empty
		assert.Equal(t, docker.DefaultPidsLimit, result.pidsLimit)
		assert.Equal(t, docker.DefaultCPUs, result.cpus)
		assert.Equal(t, docker.DefaultMemory, result.memory)
	})

	t.Run("rc takes precedence over env var", func(t *testing.T) {
		// Arrange
		t.Setenv(config.EnvPidsLimit, "256")
		rc := &config.AgenticRC{Run: config.RCRun{PidsLimit: "512"}}

		// Act
		result := resolveResourceLimits("", "", "", rc)

		// Assert
		assert.Equal(t, "512", result.pidsLimit)
	})
}

func Test_readOnlyMountSpecs(t *testing.T) {
	t.Run("forces read-only on a plain spec", func(t *testing.T) {
		// Act
		result := readOnlyMountSpecs([]string{"$PWD/secrets:$CONTAINER_HOME/secrets"})

		// Assert
		assert.Equal(t, []string{"$PWD/secrets:$CONTAINER_HOME/secrets:ro"}, result)
	})

	t.Run("strips a user-supplied :ro suffix before re-forcing it", func(t *testing.T) {
		// Act
		result := readOnlyMountSpecs([]string{"/a/b:/c:ro"})

		// Assert
		assert.Equal(t, []string{"/a/b:/c:ro"}, result)
	})

	t.Run("strips a user-supplied :rw suffix", func(t *testing.T) {
		// Act
		result := readOnlyMountSpecs([]string{"/a/b:/c:rw"})

		// Assert
		assert.Equal(t, []string{"/a/b:/c:ro"}, result)
	})

	t.Run("empty input yields empty output", func(t *testing.T) {
		// Act
		result := readOnlyMountSpecs(nil)

		// Assert
		assert.Empty(t, result)
	})

	t.Run("no-colon spec expands to the workspace-relative shorthand", func(t *testing.T) {
		// Act
		result := readOnlyMountSpecs([]string{".git"})

		// Assert
		assert.Equal(t, []string{"$PWD/.git:/workspace/.git:ro"}, result)
	})

	t.Run("no-colon spec with a nested path expands the same way", func(t *testing.T) {
		// Act
		result := readOnlyMountSpecs([]string{"secrets/foo"})

		// Assert
		assert.Equal(t, []string{"$PWD/secrets/foo:/workspace/secrets/foo:ro"}, result)
	})
}
