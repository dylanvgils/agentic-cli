package tools

import (
	"testing"

	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
	"github.com/stretchr/testify/assert"
)

func Test_createContainerUser(t *testing.T) {
	result := df.File{Stages: []df.Stage{
		df.NewStage(df.From{Image: "scratch"}).Add(createContainerUser("myuser")...).Build(),
	}}.Render()

	t.Run("renders host ID args", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "ARG HOST_UID=1000")
		assert.Contains(t, result, "ARG HOST_GID=1000")
	})

	t.Run("renders group and user add", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "groupadd -g ${HOST_GID} --non-unique myuser")
		assert.Contains(t, result, "useradd -l -u ${HOST_UID} -g ${HOST_GID} -m -s /bin/bash --non-unique myuser")
	})

	t.Run("renders conflict check", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "getent passwd ${HOST_UID}")
		assert.Contains(t, result, `"myuser"`)
	})
}

func Test_aptInstallRun_rendersUpdateInstallCleanup(t *testing.T) {
	// Arrange
	pkgs := []string{"curl", "wget"}

	// Act
	run := aptInstallRun(pkgs)
	result := df.File{Stages: []df.Stage{
		df.NewStage(df.From{Image: "scratch"}).Add(run).Build(),
	}}.Render()

	// Assert
	assert.Contains(t, result, "apt-get update -yq")
	assert.Contains(t, result, "apt-get install -yq --no-install-recommends")
	assert.Contains(t, result, "curl")
	assert.Contains(t, result, "wget")
	assert.Contains(t, result, "rm -rf /var/lib/apt/lists/*")
}

func renderStage(stage df.Stage) string {
	return df.File{Stages: []df.Stage{stage}}.Render()
}
