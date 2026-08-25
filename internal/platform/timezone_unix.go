//go:build !windows

package platform

import (
	"bytes"
	"os"
	"strings"
)

// timezone resolves the host's IANA timezone name from /etc/timezone, the /etc/localtime symlink target, or its POSIX TZ footer as a last resort.
func timezone() string {
	if tz := readEtcTimezone(); tz != "" {
		return tz
	}
	if tz := parseLocaltimeSymlink(readEtcLocaltimeTarget()); tz != "" {
		return tz
	}
	return parseLocaltimePosixFooter(readEtcLocaltimeBytes())
}

func readEtcTimezone() string {
	b, err := os.ReadFile("/etc/timezone")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// parseLocaltimeSymlink extracts the IANA zone name from a resolved /etc/localtime symlink target (".../zoneinfo/America/New_York" -> "America/New_York").
func parseLocaltimeSymlink(target string) string {
	const marker = "zoneinfo/"
	if _, after, ok := strings.Cut(target, marker); ok {
		return after
	}
	return ""
}

func readEtcLocaltimeTarget() string {
	target, err := os.Readlink("/etc/localtime")
	if err != nil {
		return ""
	}
	return target
}

// parseLocaltimePosixFooter extracts the POSIX TZ string from a TZif file's footer (RFC 8536 3.3): its last line.
func parseLocaltimePosixFooter(data []byte) string {
	last := bytes.LastIndexByte(data, '\n')
	if last == -1 {
		return ""
	}

	secondLast := bytes.LastIndexByte(data[:last], '\n')
	if secondLast == -1 {
		return ""
	}

	return string(data[secondLast+1 : last])
}

func readEtcLocaltimeBytes() []byte {
	b, err := os.ReadFile("/etc/localtime")
	if err != nil {
		return nil
	}
	return b
}
