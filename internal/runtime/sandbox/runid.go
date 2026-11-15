package sandbox

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
)

// runIDPattern matches NewRunID output.
var runIDPattern = regexp.MustCompile(`^[0-9a-f]{12}$`)

// RunID keys every per-run resource name and host path.
type RunID string

// NewRunID returns a random RunID.
func NewRunID() (RunID, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate run id: %w", err)
	}
	return RunID(hex.EncodeToString(buf)), nil
}

// ParseRunID validates s as a RunID.
func ParseRunID(s string) (RunID, error) {
	if !runIDPattern.MatchString(s) {
		return "", fmt.Errorf("invalid run id %q", s)
	}
	return RunID(s), nil
}

// String returns the ID.
func (id RunID) String() string {
	return string(id)
}
