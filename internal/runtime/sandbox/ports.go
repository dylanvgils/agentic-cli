package sandbox

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// PortLoopback is the only host address ports are published on.
const PortLoopback = "127.0.0.1"

// PortMapping forwards TCP port Host on the host's loopback to port Guest in the sandbox.
type PortMapping struct {
	Host  uint16
	Guest uint16
}

// String returns the mapping as HOST:GUEST.
func (p PortMapping) String() string {
	return fmt.Sprintf("%d:%d", p.Host, p.Guest)
}

// ParsePorts parses "PORT" or "HOST:GUEST" specs, refusing a host address, a protocol other than TCP and a host port used twice.
func ParsePorts(specs []string) ([]PortMapping, error) {
	var ports []PortMapping
	seen := make(map[uint16]bool, len(specs))
	for _, spec := range specs {
		port, err := parsePort(spec)
		if err != nil {
			return nil, err
		}
		if seen[port.Host] {
			return nil, fmt.Errorf("port %q: host port %d is forwarded twice", spec, port.Host)
		}
		seen[port.Host] = true
		ports = append(ports, port)
	}
	return ports, nil
}

// CheckPortsFree refuses a host port already in use on the host's loopback. Other listen errors, like a privileged port,
// pass: the runtime may still bind it and reports its own failure. It misses a port held on another address where the OS
// lets both bind (macOS, IPv6), and one taken between the check and the run.
func CheckPortsFree(ports []PortMapping) error {
	for _, port := range ports {
		listener, err := net.Listen("tcp", net.JoinHostPort(PortLoopback, strconv.Itoa(int(port.Host))))
		if err != nil {
			if addrInUse(err) {
				return fmt.Errorf("host port %d is in use", port.Host)
			}
			continue
		}
		_ = listener.Close()
	}
	return nil
}

// parsePort parses one "PORT" or "HOST:GUEST" spec, with an optional "/tcp".
func parsePort(spec string) (PortMapping, error) {
	ports, protocol, hasProtocol := strings.Cut(spec, "/")
	if hasProtocol && protocol != "tcp" {
		return PortMapping{}, fmt.Errorf("port %q: only TCP is forwarded", spec)
	}

	parts := strings.Split(ports, ":")
	if len(parts) > 2 || (len(parts) == 2 && isAddress(parts[0])) {
		return PortMapping{}, fmt.Errorf("port %q: ports bind to %s; drop the address", spec, PortLoopback)
	}

	host, err := parsePortNumber(spec, parts[0])
	if err != nil || len(parts) == 1 {
		return PortMapping{Host: host, Guest: host}, err
	}
	guest, err := parsePortNumber(spec, parts[1])
	return PortMapping{Host: host, Guest: guest}, err
}

// parsePortNumber parses one port number of spec, from 1 to 65535.
func parsePortNumber(spec, value string) (uint16, error) {
	n, err := strconv.ParseUint(value, 10, 16)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("port %q: %q must be a number from 1 to 65535", spec, value)
	}
	return uint16(n), nil
}

// isAddress reports whether value is a host address rather than a port.
func isAddress(value string) bool {
	return value == "localhost" || net.ParseIP(value) != nil
}

// addrInUse reports whether err is a listen failing on an address already in use.
func addrInUse(err error) bool {
	return errors.Is(err, errAddrInUse)
}
