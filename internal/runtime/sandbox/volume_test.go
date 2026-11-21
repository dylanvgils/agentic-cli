package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVolumeMount(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []VolumeOptions
		want string
	}{
		{name: "no options", want: "/host/path:/container/path"},
		{name: "read only", opts: []VolumeOptions{{ReadOnly: true}}, want: "/host/path:/container/path:ro"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := VolumeMount("/host/path", "/container/path", tc.opts...)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func Test_splitMountHost(t *testing.T) {
	for _, tc := range []struct {
		name     string
		spec     string
		wantHost string
		wantRest string
	}{
		{name: "unix path", spec: "/host/path:/container", wantHost: "/host/path", wantRest: ":/container"},
		{name: "named volume", spec: "maven:/container", wantHost: "maven", wantRest: ":/container"},
		{name: "windows drive letter", spec: `C:\Users\foo:/container`, wantHost: `C:\Users\foo`, wantRest: ":/container"},
		{name: "windows drive letter lowercase", spec: `c:\data:/container`, wantHost: `c:\data`, wantRest: ":/container"},
		{name: "no colon", spec: "maven", wantHost: "maven"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			host, rest := splitMountHost(tc.spec)

			// Assert
			assert.Equal(t, tc.wantHost, host)
			assert.Equal(t, tc.wantRest, rest)
		})
	}
}

func TestParseMount(t *testing.T) {
	pwd, err := os.Getwd()
	require.NoError(t, err)
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	relative := "." + string(filepath.Separator) + "data"

	for _, tc := range []struct {
		name string
		spec string
		want Mount
	}{
		{name: "bind mount", spec: "/host/path:/container", want: Mount{Source: "/host/path", Target: "/container"}},
		{name: "named volume", spec: "maven:/container", want: Mount{Source: "maven", Target: "/container", Named: true}},
		{name: "ro becomes ReadOnly", spec: "/host:/container:ro", want: Mount{Source: "/host", Target: "/container", ReadOnly: true}},
		{
			name: "other options kept in order",
			spec: "/host:/container:rw,z",
			want: Mount{Source: "/host", Target: "/container", Options: []string{"rw", "z"}},
		},
		{name: "trailing colon has no options", spec: "/host:/container:", want: Mount{Source: "/host", Target: "/container"}},
		{name: "TOOL_HOME", spec: "$TOOL_HOME/data:/data", want: Mount{Source: "/tool/data", Target: "/data"}},
		{name: "TOOL_HOME with braces", spec: "${TOOL_HOME}/data:/data", want: Mount{Source: "/tool/data", Target: "/data"}},
		{name: "CONTAINER_HOME", spec: "/data:$CONTAINER_HOME/data", want: Mount{Source: "/data", Target: "/root/data"}},
		{name: "CONTAINER_HOME with braces", spec: "/data:${CONTAINER_HOME}/data", want: Mount{Source: "/data", Target: "/root/data"}},
		{name: "PWD", spec: "$PWD:/workspace", want: Mount{Source: pwd, Target: "/workspace"}},
		{name: "braced PWD", spec: "${PWD}:/workspace", want: Mount{Source: pwd, Target: "/workspace"}},
		{name: "tilde", spec: "~/.cache:/cache", want: Mount{Source: home + "/.cache", Target: "/cache"}},
		{name: "HOME", spec: "$HOME/.cache:/cache", want: Mount{Source: home + "/.cache", Target: "/cache"}},
		{name: "HOME with braces", spec: "${HOME}/.cache:/cache", want: Mount{Source: home + "/.cache", Target: "/cache"}},
		{
			name: "CONTAINER_HOME in host part not expanded",
			spec: "/x/$CONTAINER_HOME:/data",
			want: Mount{Source: "/x/$CONTAINER_HOME", Target: "/data"},
		},
		{name: "TOOL_HOME in container part not expanded", spec: "/host:$TOOL_HOME/data", want: Mount{Source: "/host", Target: "$TOOL_HOME/data"}},
		{name: "redundant slashes on host side cleaned", spec: "/host//data:/data", want: Mount{Source: "/host/data", Target: "/data"}},
		{name: "container side untouched", spec: "/host:/container//path", want: Mount{Source: "/host", Target: "/container//path"}},
		{name: "dot-slash path stays a bind mount", spec: "./data:/data", want: Mount{Source: relative, Target: "/data"}},
		{name: "dot-dot path stays a bind mount", spec: "other/../data:/data", want: Mount{Source: relative, Target: "/data"}},
		{name: "windows drive path", spec: `C:\data:/container`, want: Mount{Source: filepath.Clean(`C:\data`), Target: "/container"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result, err := ParseMount(tc.spec, "/tool", "/root")

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.want, result)
		})
	}

	for _, tc := range []struct {
		name string
		spec string
	}{
		{name: "no container part", spec: "/host/only"},
		{name: "empty container part", spec: "/host:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			_, err := ParseMount(tc.spec, "/tool", "/root")

			// Assert
			assert.EqualError(t, err, `invalid volume "`+tc.spec+`": expected host:container[:options]`)
		})
	}
}

func TestParseUserMount(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec string
		want Mount
	}{
		{name: "bind mount becomes read-only", spec: "/host:/a", want: Mount{Source: "/host", Target: "/a", ReadOnly: true}},
		{name: "rw stays read-write", spec: "/host:/a:rw", want: Mount{Source: "/host", Target: "/a", Options: []string{"rw"}}},
		{name: "rw among other options stays read-write", spec: "/host:/a:z,rw", want: Mount{Source: "/host", Target: "/a", Options: []string{"z", "rw"}}},
		{name: "other options become read-only", spec: "/host:/a:z", want: Mount{Source: "/host", Target: "/a", ReadOnly: true, Options: []string{"z"}}},
		{name: "named volume stays read-write", spec: "maven:/a", want: Mount{Source: "maven", Target: "/a", Named: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result, err := ParseUserMount(tc.spec, "/tool", "/root")

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestExpandTmpfsSpec(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec string
		want string
	}{
		{name: "CONTAINER_HOME", spec: "$CONTAINER_HOME/.cache:exec,size=1g", want: "/root/.cache:exec,size=1g"},
		{name: "CONTAINER_HOME with braces", spec: "${CONTAINER_HOME}/.cache:exec,size=1g", want: "/root/.cache:exec,size=1g"},
		{name: "no options", spec: "$CONTAINER_HOME/.cache", want: "/root/.cache"},
		{name: "no placeholders", spec: "/tmp/cache:exec,size=512m", want: "/tmp/cache:exec,size=512m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := ExpandTmpfsSpec(tc.spec, "/root")

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestIsUNCPath(t *testing.T) {
	for _, tc := range []struct {
		name string
		path string
		want bool
	}{
		{name: "backslash returns true", path: `\\server\share\project`, want: true},
		{name: "forward slash returns true", path: "//server/share/project", want: true},
		{name: "local path returns false", path: "/home/user/project"},
		{name: "windows drive returns false", path: `C:\Users\foo`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := IsUNCPath(tc.path)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}
