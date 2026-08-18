package upgradecheck

import (
	"os"
	"reflect"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/selfupdate"
	"github.com/stretchr/testify/assert"
)

func TestDefaultDeps(t *testing.T) {
	t.Run("LatestVersion defaults to selfupdate.LatestVersion", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(selfupdate.LatestVersion).Pointer(), reflect.ValueOf(LatestVersion).Pointer())
	})

	t.Run("Update defaults to selfupdate.Update", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(selfupdate.Update).Pointer(), reflect.ValueOf(Update).Pointer())
	})

	t.Run("IsTerminal defaults to platform.IsTerminal", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(platform.IsTerminal).Pointer(), reflect.ValueOf(IsTerminal).Pointer())
	})

	t.Run("Exit defaults to os.Exit", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(os.Exit).Pointer(), reflect.ValueOf(Exit).Pointer())
	})
}
