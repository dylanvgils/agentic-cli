package docker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seccompRule mirrors the fields of a profile rule the tests inspect.
type seccompRule struct {
	Names    []string `json:"names"`
	Action   string   `json:"action"`
	ErrnoRet *int     `json:"errnoRet"`
	Includes struct {
		Caps []string `json:"caps"`
	} `json:"includes"`
}

// seccompProfile mirrors the top-level fields of a profile the tests inspect.
type seccompProfile struct {
	DefaultAction string        `json:"defaultAction"`
	Syscalls      []seccompRule `json:"syscalls"`
}

func Test_dindSeccompProfile(t *testing.T) {
	content, err := dindSeccompProfile()
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
		for _, name := range dindSeccompDropped {
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
		require.NoError(t, json.Unmarshal(dindSeccompDefault, &base))

		// Act
		var missing []string
		for _, rule := range base.Syscalls {
			for _, name := range rule.Names {
				if slices.Contains(dindSeccompDropped, name) {
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

func Test_writeDindSeccompProfile(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "seccomp.json")

	// Act
	err := writeDindSeccompProfile(path)

	// Assert
	require.NoError(t, err)
	content, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.True(t, json.Valid(content))
}

// allowedAnywhere reports whether any allow rule in profile names syscall.
func allowedAnywhere(profile seccompProfile, syscall string) bool {
	_, found := findRule(profile, syscall, "SCMP_ACT_ALLOW")
	return found
}

// allowedWithSysAdmin reports whether syscall is allowed unconditionally or when CAP_SYS_ADMIN is held.
func allowedWithSysAdmin(profile seccompProfile, syscall string) bool {
	for _, rule := range profile.Syscalls {
		if rule.Action != "SCMP_ACT_ALLOW" || !slices.Contains(rule.Names, syscall) {
			continue
		}
		if len(rule.Includes.Caps) == 0 || slices.Contains(rule.Includes.Caps, "CAP_SYS_ADMIN") {
			return true
		}
	}
	return false
}

// findRule returns the first rule with action that names syscall.
func findRule(profile seccompProfile, syscall, action string) (seccompRule, bool) {
	for _, rule := range profile.Syscalls {
		if rule.Action == action && slices.Contains(rule.Names, syscall) {
			return rule, true
		}
	}
	return seccompRule{}, false
}
