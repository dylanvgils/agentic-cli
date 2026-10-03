package dind

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_deriveSeccompProfile(t *testing.T) {
	content, err := deriveSeccompProfile()
	require.NoError(t, err)
	var profile seccompProfile
	require.NoError(t, json.Unmarshal(content, &profile))

	t.Run("keeps the default deny action", func(t *testing.T) {
		// Act
		result := profile.DefaultAction

		// Assert
		assert.Equal(t, "SCMP_ACT_ERRNO", result)
	})

	t.Run("still allows the namespace and mount syscalls rootless dockerd needs", func(t *testing.T) {
		for _, name := range []string{"mount", "umount2", "unshare", "setns", "clone", "pivot_root"} {
			// Act
			allowed := allowedWithSysAdmin(profile, name)

			// Assert
			assert.True(t, allowed, name)
		}
	})

	t.Run("never allows dropped kernel interfaces", func(t *testing.T) {
		for _, name := range seccompDropped {
			// Act
			allowed := allowedAnywhere(profile, name)

			// Assert
			assert.False(t, allowed, name)
		}
	})

	t.Run("keyring syscalls fail with ENOSYS rather than being allowed", func(t *testing.T) {
		for _, name := range []string{"keyctl", "add_key", "request_key"} {
			// Act
			rule, found := findRule(profile, name, "SCMP_ACT_ERRNO")

			// Assert
			require.True(t, found, name)
			require.NotNil(t, rule.ErrnoRet, name)
			assert.Equal(t, errnoENOSYS, *rule.ErrnoRet, name)
			assert.False(t, allowedAnywhere(profile, name), name)
		}
	})

	t.Run("leaves the vendored default untouched otherwise", func(t *testing.T) {
		// Arrange
		var base seccompProfile
		require.NoError(t, json.Unmarshal(seccompDefault, &base))

		// Act
		var missing []string
		for _, rule := range base.Syscalls {
			for _, name := range rule.Names {
				if slices.Contains(seccompDropped, name) {
					continue
				}
				if _, found := findRule(profile, name, rule.Action); !found {
					missing = append(missing, rule.Action+" "+name)
				}
			}
		}

		// Assert
		assert.Empty(t, missing, "every non-dropped syscall should keep its original rule")
	})
}

func TestWriteSeccompProfile(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "seccomp.json")

	// Act
	err := WriteSeccompProfile(path)

	// Assert
	require.NoError(t, err)
	content, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.True(t, json.Valid(content))
}
