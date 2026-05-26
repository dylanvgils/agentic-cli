package tools

import (
	"testing"

	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
	"github.com/stretchr/testify/assert"
)

// --- CreateContainerUser ---
func TestCreateContainerUser_rendersHostIDArgs(t *testing.T) {
	// Act
	result := df.File{Stages: []df.Stage{
		df.NewStage(df.From{Image: "scratch"}).Add(CreateContainerUser("myuser")...).Build(),
	}}.Render()

	// Assert
	assert.Contains(t, result, "ARG HOST_UID=1000")
	assert.Contains(t, result, "ARG HOST_GID=1000")
}

func TestCreateContainerUser_rendersGroupAndUserAdd(t *testing.T) {
	// Act
	result := df.File{Stages: []df.Stage{
		df.NewStage(df.From{Image: "scratch"}).Add(CreateContainerUser("myuser")...).Build(),
	}}.Render()

	// Assert
	assert.Contains(t, result, "groupadd -g ${HOST_GID} --non-unique myuser")
	assert.Contains(t, result, "useradd -l -u ${HOST_UID} -g ${HOST_GID} -m -s /bin/bash --non-unique myuser")
}

func TestCreateContainerUser_rendersConflictCheck(t *testing.T) {
	// Act
	result := df.File{Stages: []df.Stage{
		df.NewStage(df.From{Image: "scratch"}).Add(CreateContainerUser("myuser")...).Build(),
	}}.Render()

	// Assert
	assert.Contains(t, result, "getent passwd ${HOST_UID}")
	assert.Contains(t, result, `"myuser"`)
}

// --- AptInstallRun ---
func TestAptInstallRun_rendersUpdateInstallCleanup(t *testing.T) {
	// Arrange
	pkgs := []string{"curl", "wget"}

	// Act
	run := AptInstallRun(pkgs)
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
