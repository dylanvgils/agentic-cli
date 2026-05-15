package docker

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func mockInspectEnv(t *testing.T, output []byte, err error) func() {
	t.Helper()
	orig := dockerInspectEnv
	dockerInspectEnv = func(_ string) ([]byte, error) { return output, err }
	return func() { dockerInspectEnv = orig }
}

func TestResolveContainerHome_found(t *testing.T) {
	restore := mockInspectEnv(t, []byte(`["PATH=/usr/bin","TOOL_HOME=/home/claude"]`), nil)
	defer restore()

	assert.Equal(t, "/home/claude", ResolveContainerHome("agentic-claude"))
}

func TestResolveContainerHome_firstMatch(t *testing.T) {
	restore := mockInspectEnv(t, []byte(`["TOOL_HOME=/home/claude","OTHER=value","TOOL_HOME=/other"]`), nil)
	defer restore()

	assert.Equal(t, "/home/claude", ResolveContainerHome("agentic-claude"))
}

func TestResolveContainerHome_notPresent(t *testing.T) {
	restore := mockInspectEnv(t, []byte(`["PATH=/usr/bin","HOME=/root"]`), nil)
	defer restore()

	assert.Equal(t, "/root", ResolveContainerHome("agentic-claude"))
}

func TestResolveContainerHome_emptyEnv(t *testing.T) {
	restore := mockInspectEnv(t, []byte(`[]`), nil)
	defer restore()

	assert.Equal(t, "/root", ResolveContainerHome("agentic-claude"))
}

func TestResolveContainerHome_dockerError(t *testing.T) {
	restore := mockInspectEnv(t, nil, errors.New("image not found"))
	defer restore()

	assert.Equal(t, "/root", ResolveContainerHome("agentic-missing"))
}

func TestResolveContainerHome_malformedJSON(t *testing.T) {
	restore := mockInspectEnv(t, []byte(`not json`), nil)
	defer restore()

	assert.Equal(t, "/root", ResolveContainerHome("agentic-claude"))
}
