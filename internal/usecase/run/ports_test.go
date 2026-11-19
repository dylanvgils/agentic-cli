package run

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dylanvgils/agentic-cli/internal/config"
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

	t.Run("refuses a host port used twice across flag and config", func(t *testing.T) {
		// Arrange
		req := runtimeRequest("")
		req.Flags.Ports = []string{"3000"}
		req.Project.RC = &config.AgenticRC{Run: config.RCRun{Ports: []string{"3000:80"}}}

		// Act
		err := checkPorts(req)

		// Assert
		assert.EqualError(t, err, `port "3000:80": host port 3000 is forwarded twice`)
	})

	t.Run("refuses a port in use", func(t *testing.T) {
		// Arrange
		port := busyPort(t)
		req := runtimeRequest("")
		req.Project.RC = &config.AgenticRC{Run: config.RCRun{Ports: []string{strconv.Itoa(int(port))}}}

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
