package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateDindDockerfile(t *testing.T) {
	t.Run("builds on the upstream rootless image", func(t *testing.T) {
		// Act
		result := GenerateDindDockerfile("")

		// Assert
		assert.Contains(t, result, "FROM docker:"+DefaultVersions.Docker+"-dind-rootless AS dind")
	})

	t.Run("registry prefixes the base image", func(t *testing.T) {
		// Act
		result := GenerateDindDockerfile("mirror.example.com/")

		// Assert
		assert.Contains(t, result, "FROM mirror.example.com/docker:"+DefaultVersions.Docker+"-dind-rootless AS dind")
	})

	t.Run("replaces setuid with narrow file capabilities and verifies none remain", func(t *testing.T) {
		// Act
		result := GenerateDindDockerfile("")

		// Assert
		assert.Contains(t, result, "-perm /6000 -exec chmod u-s,g-s {} +")
		assert.Contains(t, result, "setcap cap_setuid=ep /usr/bin/newuidmap")
		assert.Contains(t, result, "setcap cap_setgid=ep /usr/bin/newgidmap")
		assert.Contains(t, result, `test -z "$(find / -xdev -type f -perm /6000)"`)
		assert.Contains(t, result, "chmod 1777 /home/rootless/.local/share/docker")
	})

	t.Run("drops back to the rootless user", func(t *testing.T) {
		// Act
		result := GenerateDindDockerfile("")

		// Assert
		assert.Regexp(t, `USER rootless\s*$`, result)
	})
}
