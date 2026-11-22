package agenticrc

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config/agenticrc/agenticrctest"
	"github.com/stretchr/testify/assert"
)

func TestConfig_GuardedSettings(t *testing.T) {
	t.Run("returns each guarded key with its value", func(t *testing.T) {
		// Arrange
		disabled := false
		rc := &Config{}
		rc.Run.Mounts = []string{"~/.example:/x:rw"}
		rc.Run.Runtime = "microsandbox"
		rc.Run.Args = map[string][]string{"copilot": {"--example"}}
		rc.Run.Proxy.Enabled = &disabled
		rc.Run.Proxy.Backend = BackendAgentic
		rc.Run.Proxy.Credentials = []Credential{{Preset: "example", Secret: "/example.test/key"}}

		// Act
		settings := rc.GuardedSettings()

		// Assert
		assert.Contains(t, settings, GuardedSetting{"run.mounts", []string{"~/.example:/x:rw"}})
		assert.Contains(t, settings, GuardedSetting{"run.runtime", "microsandbox"})
		assert.Contains(t, settings, GuardedSetting{"run.args", rc.Run.Args})
		assert.Contains(t, settings, GuardedSetting{"run.proxy.enabled", &disabled})
		assert.Contains(t, settings, GuardedSetting{"run.proxy.backend", BackendAgentic})
		assert.Contains(t, settings, GuardedSetting{CredentialsSetting, rc.Run.Proxy.Credentials})
	})

	t.Run("classifies every config key as guarded or deliberately unguarded", func(t *testing.T) {
		// Arrange
		// These stay inside the container or only shape the image, so changing them needs no approval
		unguarded := []string{
			// Build-only, inside the image
			"build.bases",
			"build.apt_packages",
			"build.versions",
			// Resource limits only shrink what the container gets
			"run.pids_limit",
			"run.cpus",
			"run.memory",
			"run.dind.pids_limit",
			"run.dind.cpus",
			"run.dind.memory",
			// Host-side behaviour that grants nothing
			"run.check_updates",
			"run.instructions.enabled",
			"run.instructions.custom",
			// Only points tools at agentic's proxy
			"run.proxy.integrations",
		}

		// Act
		settings := (&Config{}).GuardedSettings()

		// Assert
		var classified []string
		for _, s := range settings {
			classified = append(classified, s.Key)
		}
		classified = append(classified, unguarded...)
		assert.ElementsMatch(t, agenticrctest.Keys[Config](), classified,
			"a new config key must be added to Config.GuardedSettings if it reaches past the container, else to unguarded here")
	})
}

func Test_GuardedSetting_IsSet(t *testing.T) {
	disabled := false

	t.Run("nil and empty values are unset", func(t *testing.T) {
		// Act
		nilList := GuardedSetting{"run.env", []string(nil)}.IsSet()
		emptyList := GuardedSetting{"run.env", []string{}}.IsSet()
		nilBool := GuardedSetting{"run.dind.enabled", (*bool)(nil)}.IsSet()
		emptyString := GuardedSetting{"namespace", ""}.IsSet()

		// Assert
		assert.False(t, nilList)
		assert.False(t, emptyList)
		assert.False(t, nilBool)
		assert.False(t, emptyString)
	})

	t.Run("explicit false is set", func(t *testing.T) {
		// Act
		set := GuardedSetting{"run.proxy.enabled", &disabled}.IsSet()

		// Assert
		assert.True(t, set)
	})
}

func Test_GuardedSetting_hash(t *testing.T) {
	t.Run("unset values hash the same", func(t *testing.T) {
		// Act
		empty := GuardedSetting{"run.env", []string{}}.hash()
		missing := GuardedSetting{}.hash()

		// Assert
		assert.Equal(t, missing, empty)
	})

	t.Run("different values hash differently", func(t *testing.T) {
		// Act
		first := GuardedSetting{"run.env", []string{"A"}}.hash()
		second := GuardedSetting{"run.env", []string{"B"}}.hash()

		// Assert
		assert.NotEqual(t, first, second)
	})
}

func TestLayer_ApprovalHashes(t *testing.T) {
	// Arrange
	layer := Layer{Path: "/example.test/.agenticrc.toml", Config: &Config{Namespace: "work"}}

	// Act
	hashes := layer.ApprovalHashes()

	// Assert
	assert.Equal(t, map[string]string{"namespace": GuardedSetting{"namespace", "work"}.hash()}, hashes)
}

func TestLayer_ChangedSettings(t *testing.T) {
	mounts := func(m ...string) Layer {
		rc := &Config{}
		rc.Run.Mounts = m
		return Layer{Path: "/example.test/.agenticrc.toml", Config: rc}
	}

	t.Run("unset setting never approved is unchanged", func(t *testing.T) {
		// Act
		changed := mounts().ChangedSettings(nil)

		// Assert
		assert.Empty(t, changed)
	})

	t.Run("new setting is changed", func(t *testing.T) {
		// Act
		changed := mounts("~/.example:/x:rw").ChangedSettings(nil)

		// Assert
		assert.Equal(t, []GuardedSetting{{"run.mounts", []string{"~/.example:/x:rw"}}}, changed)
	})

	t.Run("approved setting is unchanged", func(t *testing.T) {
		// Arrange
		approved := mounts("~/.example:/x:rw").ApprovalHashes()

		// Act
		changed := mounts("~/.example:/x:rw").ChangedSettings(approved)

		// Assert
		assert.Empty(t, changed)
	})

	t.Run("cleared approved setting is changed", func(t *testing.T) {
		// Arrange
		approved := mounts("~/.example:/x:rw").ApprovalHashes()

		// Act
		changed := mounts().ChangedSettings(approved)

		// Assert
		assert.Equal(t, []GuardedSetting{{"run.mounts", []string(nil)}}, changed)
	})
}
