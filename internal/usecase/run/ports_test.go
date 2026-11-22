package run

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dylanvgils/agentic-cli/internal/config/rc"
)

func Test_checkPorts(t *testing.T) {
	t.Run("passes free ports", func(t *testing.T) {
		// Arrange
		req := runtimeRequest("")
		req.Flags.Ports = []string{strconv.Itoa(int(freePort(t)))}

		// Act
		err := checkPorts(req)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("refuses an invalid port", func(t *testing.T) {
		// Arrange
		req := runtimeRequest("")
		req.Flags.Ports = []string{"http"}

		// Act
		err := checkPorts(req)

		// Assert
		assert.EqualError(t, err, `port "http": "http" must be a number from 1 to 65535`)
	})

	t.Run("refuses a port in use", func(t *testing.T) {
		// Arrange
		port := busyPort(t)
		req := runtimeRequest("")
		req.Project.RC = &rc.AgenticRC{Run: rc.RCRun{Ports: []string{strconv.Itoa(int(port))}}}

		// Act
		err := checkPorts(req)

		// Assert
		assert.EqualError(t, err, fmt.Sprintf("host port %d is in use", port))
	})

	t.Run("a dry run skips the port in use check", func(t *testing.T) {
		// Arrange
		req := runtimeRequest("")
		req.Flags.DryRun = true
		req.Flags.Ports = []string{strconv.Itoa(int(busyPort(t)))}

		// Act
		err := checkPorts(req)

		// Assert
		assert.NoError(t, err)
	})
}
