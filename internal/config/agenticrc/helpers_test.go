package agenticrc

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParseRC(t *testing.T, content string) *Config {
	t.Helper()
	path := writeRC(t, content)
	rc, err := loadRC(path)
	require.NoError(t, err)
	return rc
}

func writeRC(t *testing.T, content string) string {
	t.Helper()
	return writeRCIn(t, t.TempDir(), content)
}

// writeRCIn writes content as dir's .agenticrc.toml and returns its path.
func writeRCIn(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, ".agenticrc.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// nestedDirs returns a temp dir and depth dirs nested below it, outermost first.
func nestedDirs(t *testing.T, depth int) []string {
	t.Helper()
	dirs := []string{t.TempDir()}
	for range depth {
		dir := filepath.Join(dirs[len(dirs)-1], "sub")
		require.NoError(t, os.Mkdir(dir, 0o755))
		dirs = append(dirs, dir)
	}
	return dirs
}

// scalarsOf returns rc with only the settings where the innermost layer wins.
func scalarsOf(rc *Config) Config {
	return Config{
		Namespace:     rc.Namespace,
		DockerContext: rc.DockerContext,
		Run: Run{
			Runtime:      rc.Run.Runtime,
			Proxy:        Proxy{Enabled: rc.Run.Proxy.Enabled, Mode: rc.Run.Proxy.Mode, Backend: rc.Run.Proxy.Backend},
			Dind:         rc.Run.Dind,
			Instructions: Instructions{Enabled: rc.Run.Instructions.Enabled},
			Limits:       rc.Run.Limits,
			CheckUpdates: rc.Run.CheckUpdates,
		},
	}
}

// isUnset reports whether a setting's value is zero; an empty list or map counts as unset.
func isUnset(v reflect.Value) bool {
	if v.Kind() == reflect.Slice || v.Kind() == reflect.Map {
		return v.Len() == 0
	}
	return v.IsZero()
}

// assertMerged checks that v, merged from an "outer" and an "inner" filled layer, holds both layers' list entries and
// the inner scalars; map values are checked per entry.
func assertMerged(t *testing.T, key string, v reflect.Value) {
	t.Helper()
	switch v.Kind() {
	case reflect.Slice:
		assert.Equal(t, 2, v.Len(), "%s should hold the entries of both layers", key)
	case reflect.Map:
		for entry := v.MapRange(); entry.Next(); {
			assertMerged(t, key+"."+entry.Key().String(), entry.Value())
		}
	case reflect.Pointer:
		if assert.False(t, v.IsNil(), "%s was set in both layers but lost in the merge", key) {
			assertMerged(t, key, v.Elem())
		}
	case reflect.String:
		assert.Equal(t, "inner", v.String(), "%s should take the inner layer's value", key)
	default:
		assert.False(t, v.IsZero(), "%s was set in both layers but lost in the merge", key)
	}
}
