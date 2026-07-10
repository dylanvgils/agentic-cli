package platform

import "os"

// Timezone returns the host's IANA timezone name (e.g. "America/New_York"),
// or "" if it can't be determined. An explicit TZ env var always wins.
func Timezone() string {
	if tz := os.Getenv("TZ"); tz != "" {
		return tz
	}
	return timezone()
}
