package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateDindDockerfile(t *testing.T) {
	for _, tc := range []struct {
		name     string
		registry string
		wantFrom string
	}{
		{
			name:     "builds on the upstream rootless image",
			registry: "",
			wantFrom: "FROM docker:" + DefaultVersions.Docker + "-dind-rootless AS dind",
		},
		{
			name:     "registry prefixes the base image",
			registry: "mirror.example.com/",
			wantFrom: "FROM mirror.example.com/docker:" + DefaultVersions.Docker + "-dind-rootless AS dind",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := GenerateDindDockerfile(tc.registry)

			// Assert
			assert.Contains(t, result, tc.wantFrom)
		})
	}

	t.Run("replaces setuid with narrow file capabilities and drops back to the rootless user", func(t *testing.T) {
		// Act
		result := GenerateDindDockerfile("")

		// Assert
		assert.Contains(t, result, "-perm /6000 -exec chmod u-s,g-s {} +")
		assert.Contains(t, result, "setcap cap_setuid=ep /usr/bin/newuidmap")
		assert.Contains(t, result, "setcap cap_setgid=ep /usr/bin/newgidmap")
		assert.Contains(t, result, `test -z "$(find / -xdev -type f -perm /6000)"`)
		assert.Contains(t, result, "chmod 1777 /home/rootless/.local/share/docker")
		assert.Regexp(t, `USER rootless\s*$`, result)
	})
}
