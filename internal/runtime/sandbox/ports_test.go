package sandbox

import (
	"fmt"
	"net"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortMapping_String(t *testing.T) {
	// Act
	result := PortMapping{Host: 8080, Guest: 80}.String()

	// Assert
	assert.Equal(t, "8080:80", result)
}

func TestParsePorts(t *testing.T) {
	t.Run("parses single ports and host:guest pairs", func(t *testing.T) {
		// Act
		ports, err := ParsePorts([]string{
			"3000",
			"8080:80",
			"65535:1",
		})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []PortMapping{
			{Host: 3000, Guest: 3000},
			{Host: 8080, Guest: 80},
			{Host: 65535, Guest: 1},
		}, ports)
	})

	for _, tc := range []struct {
		name  string
		specs []string
		want  string
	}{
		{"host address", []string{"0.0.0.0:8080:80"}, `port "0.0.0.0:8080:80": ports bind to 127.0.0.1; drop the address`},
		{"protocol", []string{"8080:80/udp"}, `port "8080:80/udp": only TCP is forwarded; drop the protocol`},
		{"zero", []string{"0"}, `port "0": "0" must be a number from 1 to 65535`},
		{"above range", []string{"8080:65536"}, `port "8080:65536": "65536" must be a number from 1 to 65535`},
		{"not a number", []string{"http"}, `port "http": "http" must be a number from 1 to 65535`},
		{"range", []string{"8000-8002:80-82"}, `port "8000-8002:80-82": "8000-8002" must be a number from 1 to 65535`},
		{"empty", []string{""}, `port "": "" must be a number from 1 to 65535`},
		{"duplicate host port", []string{"8080:80", "8080:81"}, `port "8080:81": host port 8080 is forwarded twice`},
	} {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			// Act
			_, err := ParsePorts(tc.specs)

			// Assert
			assert.EqualError(t, err, tc.want)
		})
	}
}

func TestCheckPortsFree(t *testing.T) {
	t.Run("passes a free port", func(t *testing.T) {
		// Arrange
		listener, port := listenLoopback(t)
		require.NoError(t, listener.Close())

		// Act
		err := CheckPortsFree([]PortMapping{{Host: port, Guest: 80}})

		// Assert
		assert.NoError(t, err)
	})

	t.Run("refuses a port in use", func(t *testing.T) {
		// Arrange
		_, port := listenLoopback(t)

		// Act
		err := CheckPortsFree([]PortMapping{{Host: port, Guest: 80}})

		// Assert
		assert.EqualError(t, err, fmt.Sprintf("host port %d is in use", port))
	})
}

func Test_addrInUse(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "an address in use",
			err:  &net.OpError{Op: "listen", Err: os.NewSyscallError("bind", errAddrInUse)},
			want: true,
		},
		{
			name: "another listen error",
			err:  &net.OpError{Op: "listen", Err: os.NewSyscallError("bind", syscall.EACCES)},
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got := addrInUse(tc.err)

			// Assert
			assert.Equal(t, tc.want, got)
		})
	}
}
