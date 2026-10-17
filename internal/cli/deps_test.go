package cli

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/microsandbox"
	"github.com/stretchr/testify/assert"
)

var _ dockerAPI = (*docker.Client)(nil)

func Test_msbClient(t *testing.T) {
	// Assert
	assert.IsType(t, &microsandbox.Client{}, msbClient)
}
