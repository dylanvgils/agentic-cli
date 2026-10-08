package toolupdate

import (
	"reflect"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
)

var _ Docker = (*docker.Client)(nil)

func TestDefaultDeps(t *testing.T) {
	// Assert
	assert.Equal(t, reflect.ValueOf(docker.LatestToolVersion).Pointer(), reflect.ValueOf(LatestToolVersion).Pointer())
}
