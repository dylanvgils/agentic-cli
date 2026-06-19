package proxy

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoggerLog(t *testing.T) {
	t.Run("writes one JSON line per call to the file destination", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		logger := NewLogger(&buf, nil, nil)
		logger.now = func() time.Time { return time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC) }

		// Act
		logger.Log(ProtocolHTTPS, "api.anthropic.com", "443", DecisionAllow)
		logger.Log(ProtocolHTTP, "evil.com", "443", DecisionDeny)

		// Assert
		lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
		require.Len(t, lines, 2)

		var allow Entry
		require.NoError(t, json.Unmarshal(lines[0], &allow))
		assert.Equal(t, ProtocolHTTPS, allow.Protocol)
		assert.Equal(t, "api.anthropic.com", allow.Host)
		assert.Equal(t, "443", allow.Port)
		assert.Equal(t, DecisionAllow, allow.Decision)
		assert.Equal(t, "2026-06-17T12:00:00Z", allow.Time.Format(time.RFC3339))

		var deny Entry
		require.NoError(t, json.Unmarshal(lines[1], &deny))
		assert.Equal(t, ProtocolHTTP, deny.Protocol)
		assert.Equal(t, DecisionDeny, deny.Decision)
	})

	t.Run("writes one human-readable line per call to the human destination", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		logger := NewLogger(nil, &buf, nil)
		logger.now = func() time.Time { return time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC) }

		// Act
		logger.Log(ProtocolHTTPS, "api.anthropic.com", "443", DecisionAllow)
		logger.Log(ProtocolHTTP, "evil.com", "443", DecisionDeny)

		// Assert
		lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
		require.Len(t, lines, 2)
		assert.Equal(t, "2026-06-17T12:00:00Z [ALLOW] https api.anthropic.com:443", string(lines[0]))
		assert.Equal(t, "2026-06-17T12:00:00Z [DENY]  http  evil.com:443", string(lines[1]))
	})

	t.Run("human destination uses the configured location", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		location := time.FixedZone("", 2*60*60)
		logger := NewLogger(nil, &buf, location)
		logger.now = func() time.Time { return time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC) }

		// Act
		logger.Log(ProtocolHTTPS, "api.anthropic.com", "443", DecisionAllow)

		// Assert
		lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
		require.Len(t, lines, 1)
		assert.Equal(t, "2026-06-17T14:00:00+02:00 [ALLOW] https api.anthropic.com:443", string(lines[0]))
	})

	t.Run("nil destinations are skipped without writing", func(t *testing.T) {
		// Arrange
		logger := NewLogger(nil, nil, nil)

		// Act + Assert
		assert.NotPanics(t, func() { logger.Log(ProtocolHTTPS, "api.anthropic.com", "443", DecisionAllow) })
	})
}
