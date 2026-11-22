package agenticrc

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

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
