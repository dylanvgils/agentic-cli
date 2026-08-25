package platform

import "os"

// Timezone returns the host's IANA timezone name, or "" if undetermined. An explicit TZ env var always wins.
func Timezone() string {
	if tz := os.Getenv("TZ"); tz != "" {
		return tz
	}
	return timezone()
}
