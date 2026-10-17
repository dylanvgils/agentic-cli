package config

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func mustParseRC(t *testing.T, content string) *AgenticRC {
	t.Helper()
	path := writeRC(t, content)
	rc, err := loadRC(path)
	require.NoError(t, err)
	return rc
}

func writeRC(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".agenticrc.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// symlinkOrSkip creates link pointing at target, skipping the test where symlinks need privileges.
func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	require.NoError(t, os.Symlink(target, link))
}

// rcKeys returns the dotted toml key of every setting in config struct t: nested tables recurse, embedded structs share
// their parent's table and lists of tables count as one key.
func rcKeys(t reflect.Type, prefix string) []string {
	var keys []string
	for i := range t.NumField() {
		field := t.Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("toml"), ",")
		switch {
		case field.Anonymous:
			keys = append(keys, rcKeys(field.Type, prefix)...)
		case field.Type.Kind() == reflect.Struct:
			keys = append(keys, rcKeys(field.Type, prefix+name+".")...)
		default:
			keys = append(keys, prefix+name)
		}
	}
	return keys
}
