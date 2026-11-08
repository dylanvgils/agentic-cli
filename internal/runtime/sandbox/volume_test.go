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

func TestReadOnlyByDefault(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec string
		want string
	}{
		{"bind mount without options becomes read-only", "/host/a:/a", "/host/a:/a:ro"},
		{"bind mount with rw stays read-write", "/host/a:/a:rw", "/host/a:/a:rw"},
		{"bind mount with ro is unchanged", "/host/a:/a:ro", "/host/a:/a:ro"},
		{"other options get ro appended", "/host/a:/a:z", "/host/a:/a:z,ro"},
		{"rw among other options stays read-write", "/host/a:/a:rw,z", "/host/a:/a:rw,z"},
		{"trailing colon gets ro", "/host/a:/a:", "/host/a:/a:ro"},
		{"named volume is unchanged", "maven:$CONTAINER_HOME/.m2", "maven:$CONTAINER_HOME/.m2"},
		{"home placeholder bind mount becomes read-only", "~/.m2:$CONTAINER_HOME/.m2", "~/.m2:$CONTAINER_HOME/.m2:ro"},
		{"windows drive path becomes read-only", `C:\host\a:/a`, `C:\host\a:/a:ro`},
		{"spec without container part is unchanged", "/host/a", "/host/a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := ReadOnlyByDefault(tc.spec)

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

func TestHostPart(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec string
		want string
	}{
		{name: "unix path", spec: "/host/path:/container", want: "/host/path"},
		{name: "named volume", spec: "maven:/container", want: "maven"},
		{name: "windows drive letter", spec: `C:\Users\foo:/container`, want: `C:\Users\foo`},
		{name: "no colon", spec: "maven", want: "maven"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := HostPart(tc.spec)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestSplitHostContainer(t *testing.T) {
	for _, tc := range []struct {
		name          string
		spec          string
		wantHost      string
		wantContainer string
		wantOK        bool
	}{
		{name: "unix path with container part", spec: "/host/path:/container", wantHost: "/host/path", wantContainer: "/container", wantOK: true},
		{name: "named volume with container part", spec: "maven:/container", wantHost: "maven", wantContainer: "/container", wantOK: true},
		{name: "no colon returns ok false", spec: "maven", wantHost: "maven"},
		{name: "trailing colon returns empty container with ok true", spec: "maven:", wantHost: "maven", wantOK: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			host, container, ok := SplitHostContainer(tc.spec)

			// Assert
			assert.Equal(t, tc.wantHost, host)
			assert.Equal(t, tc.wantContainer, container)
			assert.Equal(t, tc.wantOK, ok)
		})
	}
}

func TestIsNamedVolume(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec string
		want bool
	}{
		{name: "named volume returns true", spec: "maven:/container", want: true},
		{name: "unix path returns false", spec: "/host/path:/container"},
		{name: "windows path returns false", spec: `C:\data:/container`},
		{name: "single char returns false", spec: "C:/container"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := IsNamedVolume(tc.spec)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestNormalizeMountSpec(t *testing.T) {
	relative := "." + string(filepath.Separator) + "data:/data"

	for _, tc := range []struct {
		name string
		spec string
		want string
	}{
		{name: "unix path unchanged", spec: "/host/path:/container/path", want: "/host/path:/container/path"},
		{name: "named volume unchanged", spec: "maven:/container/path", want: "maven:/container/path"},
		{name: "redundant slashes on host side cleaned", spec: "/host//data:/container/path", want: "/host/data:/container/path"},
		{name: "dot-slash path stays a bind mount", spec: "./data:/data", want: relative},
		{name: "trailing dot path stays a bind mount", spec: "data/.:/data", want: relative},
		{name: "dot-dot path stays a bind mount", spec: "other/../data:/data", want: relative},
		{name: "container side untouched", spec: "/host/data:/container//path", want: "/host/data:/container//path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := NormalizeMountSpec(tc.spec)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestExpandMountSpec(t *testing.T) {
	pwd, err := os.Getwd()
	require.NoError(t, err)
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	for _, tc := range []struct {
		name          string
		spec          string
		toolHome      string
		containerHome string
		want          string
	}{
		{name: "TOOL_HOME", spec: "$TOOL_HOME/data:/data", toolHome: "/custom/home", want: "/custom/home/data:/data"},
		{name: "TOOL_HOME with braces", spec: "${TOOL_HOME}/data:/data", toolHome: "/custom/home", want: "/custom/home/data:/data"},
		{name: "CONTAINER_HOME", spec: "/data:$CONTAINER_HOME/data", containerHome: "/root", want: "/data:/root/data"},
		{name: "CONTAINER_HOME with braces", spec: "/data:${CONTAINER_HOME}/data", containerHome: "/root", want: "/data:/root/data"},
		{name: "PWD", spec: "$PWD:/workspace", want: pwd + ":/workspace"},
		{name: "braced PWD", spec: "${PWD}:/workspace", want: pwd + ":/workspace"},
		{name: "tilde", spec: "~/.cache:/cache", want: home + "/.cache:/cache"},
		{name: "HOME", spec: "$HOME/.cache:/cache", want: home + "/.cache:/cache"},
		{name: "HOME with braces", spec: "${HOME}/.cache:/cache", want: home + "/.cache:/cache"},
		{
			name:          "CONTAINER_HOME in host part not expanded",
			spec:          "$CONTAINER_HOME/data:/container/data",
			toolHome:      "/tool",
			containerHome: "/root",
			want:          "$CONTAINER_HOME/data:/container/data",
		},
		{
			name:          "TOOL_HOME in container part not expanded",
			spec:          "/host/data:$TOOL_HOME/data",
			toolHome:      "/tool",
			containerHome: "/root",
			want:          "/host/data:$TOOL_HOME/data",
		},
		{
			name:          "no placeholders",
			spec:          "/host/path:/container/path",
			toolHome:      "/custom/home",
			containerHome: "/root",
			want:          "/host/path:/container/path",
		},
		{
			name:          "mixed",
			spec:          "$TOOL_HOME/cfg:${CONTAINER_HOME}/.config",
			toolHome:      "/home/.agentic",
			containerHome: "/root",
			want:          "/home/.agentic/cfg:/root/.config",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := ExpandMountSpec(tc.spec, tc.toolHome, tc.containerHome)

			// Assert
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
