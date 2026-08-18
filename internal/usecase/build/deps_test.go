package build

import (
	"reflect"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
)

func TestDefaultDeps(t *testing.T) {
	t.Run("BuildTool defaults to docker.BuildTool", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(docker.BuildTool).Pointer(), reflect.ValueOf(BuildTool).Pointer())
	})
}
