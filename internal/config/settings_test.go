package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGuardedSettings(t *testing.T) {
	// Arrange
	disabled := false
	rc := &AgenticRC{}
	rc.Run.ExtraMounts = []string{"~/.example:/x:rw"}
	rc.Run.Proxy.Enabled = &disabled
	rc.Run.Proxy.Credentials = []RCCredential{{Preset: "example", Secret: "/example.test/key"}}

	// Act
	settings := GuardedSettings(rc)

	// Assert
	assert.Contains(t, settings, GuardedSetting{"run.extra_mounts", []string{"~/.example:/x:rw"}})
	assert.Contains(t, settings, GuardedSetting{"run.proxy.enabled", &disabled})
	assert.Contains(t, settings, GuardedSetting{CredentialsSetting, rc.Run.Proxy.Credentials})
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
