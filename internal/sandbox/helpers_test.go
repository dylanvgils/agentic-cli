package sandbox

import "testing"

// stubHostTimezone replaces hostTimezone with a stub that returns val for the duration of the test.
func stubHostTimezone(t *testing.T, val string) {
	t.Helper()
	orig := hostTimezone
	hostTimezone = func() string { return val }
	t.Cleanup(func() { hostTimezone = orig })
}

// clearTerminalEnv unsets the forwarded terminal vars for the duration of the test.
func clearTerminalEnv(t *testing.T) {
	t.Helper()
	for _, key := range terminalEnvNames {
		t.Setenv(key, "")
	}
}
