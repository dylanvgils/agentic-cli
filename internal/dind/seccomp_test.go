package dind

import (
	"bytes"
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

	t.Run("uses only fields docker understands", func(t *testing.T) {
		// Arrange
		decoder := json.NewDecoder(bytes.NewReader(content))
		decoder.DisallowUnknownFields()

		// Act
		var strict seccompProfile
		err := decoder.Decode(&strict)

		// Assert - docker ignores unknown fields, so a typo like "include" would silently widen a rule
		assert.NoError(t, err)
	})

	t.Run("uses only known actions and comparison ops", func(t *testing.T) {
		// Arrange
		actions := []string{
			"SCMP_ACT_KILL", "SCMP_ACT_KILL_PROCESS", "SCMP_ACT_KILL_THREAD", "SCMP_ACT_TRAP",
			"SCMP_ACT_ERRNO", "SCMP_ACT_TRACE", "SCMP_ACT_ALLOW", "SCMP_ACT_LOG", "SCMP_ACT_NOTIFY",
		}
		ops := []string{
			"SCMP_CMP_NE", "SCMP_CMP_LT", "SCMP_CMP_LE", "SCMP_CMP_EQ",
			"SCMP_CMP_GE", "SCMP_CMP_GT", "SCMP_CMP_MASKED_EQ",
		}

		// Act
		var unknown []string
		for _, rule := range profile.Syscalls {
			if !slices.Contains(actions, rule.Action) {
				unknown = append(unknown, rule.Action)
			}
			for _, arg := range rule.Args {
				if !slices.Contains(ops, arg.Op) {
					unknown = append(unknown, arg.Op)
				}
			}
		}

		// Assert
		assert.Empty(t, unknown)
	})

	t.Run("keeps the default deny action", func(t *testing.T) {
		// Act
		result := profile.DefaultAction

		// Assert
		assert.Equal(t, "SCMP_ACT_ERRNO", result)
	})

	t.Run("allows pivot_root with CAP_SYS_ADMIN for nested runc", func(t *testing.T) {
		// Act
		allowed := allowedWithSysAdmin(profile, "pivot_root")

		// Assert
		assert.True(t, allowed)
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
