package precedence

import (
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config/rc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNamespace(t *testing.T) {
	agenticRC := &rc.AgenticRC{Namespace: "fromrc"}

	for _, tc := range []struct {
		name string
		flag string
		rc   *rc.AgenticRC
		want string
	}{
		{name: "flag takes priority over rc", flag: "fromflag", rc: agenticRC, want: "fromflag"},
		{name: "rc value used when flag absent", rc: agenticRC, want: "fromrc"},
		{name: "falls back to default when nothing set", want: rc.DefaultNamespace},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := Namespace(tc.flag, tc.rc)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestRegistry(t *testing.T) {
	for _, tc := range []struct {
		name   string
		flag   string
		config string
		want   string
	}{
		{name: "flag takes priority over config", flag: "flag.example.com", config: `{"registry":"config.example.com"}`, want: "flag.example.com"},
		{name: "falls back to agentic.json when flag not set", config: `{"registry":"config.example.com"}`, want: "config.example.com"},
		{name: "empty when neither set"},
		{name: "empty when agentic.json is invalid", config: "invalid json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			homeDir := t.TempDir()
			if tc.config != "" {
				require.NoError(t, os.WriteFile(rc.ConfigFile(homeDir), []byte(tc.config), 0o640))
			}

			// Act
			result := Registry(tc.flag, homeDir)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestDockerContext(t *testing.T) {
	agenticRC := &rc.AgenticRC{DockerContext: "fromrc"}
	fromConfig := `{"docker_context":"fromconfig"}`

	for _, tc := range []struct {
		name   string
		flag   string
		rc     *rc.AgenticRC
		config string
		want   string
	}{
		{name: "flag wins over everything", flag: "fromflag", rc: agenticRC, config: fromConfig, want: "fromflag"},
		{name: "rc value wins over agentic.json", rc: agenticRC, config: fromConfig, want: "fromrc"},
		{name: "agentic.json wins when rc unset", config: fromConfig, want: "fromconfig"},
		{name: "empty when nothing set"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			homeDir := t.TempDir()
			if tc.config != "" {
				require.NoError(t, os.WriteFile(rc.ConfigFile(homeDir), []byte(tc.config), 0o640))
			}

			// Act
			result := DockerContext(tc.flag, tc.rc, homeDir)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}
