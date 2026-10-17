package config

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGuardedSettings(t *testing.T) {
	t.Run("returns each guarded key with its value", func(t *testing.T) {
		// Arrange
		disabled := false
		rc := &AgenticRC{}
		rc.Run.Mounts = []string{"~/.example:/x:rw"}
		rc.Run.Proxy.Enabled = &disabled
		rc.Run.Proxy.Credentials = []RCCredential{{Preset: "example", Secret: "/example.test/key"}}

		// Act
		settings := GuardedSettings(rc)

		// Assert
		assert.Contains(t, settings, GuardedSetting{"run.mounts", []string{"~/.example:/x:rw"}})
		assert.Contains(t, settings, GuardedSetting{"run.proxy.enabled", &disabled})
		assert.Contains(t, settings, GuardedSetting{CredentialsSetting, rc.Run.Proxy.Credentials})
	})

	t.Run("classifies every config key as guarded or deliberately unguarded", func(t *testing.T) {
		// Arrange
		// These stay inside the container or only shape the image, so changing them needs no approval
		unguarded := []string{
			"build.bases", "build.apt_packages", "build.versions",
			"run.pids_limit", "run.cpus", "run.memory", "run.check_updates",
			"run.dind.pids_limit", "run.dind.cpus", "run.dind.memory",
			"run.instructions.enabled", "run.instructions.custom",
		}

		// Act
		settings := GuardedSettings(&AgenticRC{})

		// Assert
		var classified []string
		for _, s := range settings {
			classified = append(classified, s.Key)
		}
		classified = append(classified, unguarded...)
		assert.ElementsMatch(t, rcKeys(reflect.TypeFor[AgenticRC](), ""), classified,
			"a new config key must be added to GuardedSettings if it reaches past the container, else to unguarded here")
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
