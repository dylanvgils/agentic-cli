package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogger_Log(t *testing.T) {
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name     string
		entry    Entry
		wantLine string
	}{
		{
			name:     "allowed https request",
			entry:    Entry{Protocol: ProtocolHTTPS, Host: "api.example.test", Port: "443", Decision: DecisionAllow, Enforced: true},
			wantLine: "proto=https host=api.example.test port=443 decision=allow",
		},
		{
			name:     "denied http request",
			entry:    Entry{Protocol: ProtocolHTTP, Host: "evil.test", Port: "443", Decision: DecisionDeny, Enforced: true},
			wantLine: "proto=http host=evil.test port=443 decision=deny",
		},
		{
			name:     "monitor mode marks the request",
			entry:    Entry{Protocol: ProtocolHTTPS, Host: "evil.test", Port: "443", Decision: DecisionDeny},
			wantLine: "proto=https host=evil.test port=443 decision=deny monitor=true",
		},
		{
			name:     "intercepted tunnel is tagged",
			entry:    Entry{Protocol: ProtocolHTTPS, Host: "api.example.test", Port: "443", Decision: DecisionAllow, Enforced: true, Intercepted: true},
			wantLine: "proto=https host=api.example.test port=443 decision=allow intercepted=true",
		},
		{
			name:     "reason is tagged and the error stays out of the human line",
			entry:    Entry{Protocol: ProtocolHTTPS, Host: "api.example.test", Port: "443", Decision: DecisionDeny, Enforced: true, Reason: ReasonBlockedAddr, Error: "test-error"},
			wantLine: "proto=https host=api.example.test port=443 decision=deny reason=blocked-address",
		},
		{
			name:     "refused path is shown after the port",
			entry:    Entry{Protocol: ProtocolHTTPS, Host: "raw.example.test", Port: "443", Decision: DecisionDeny, Enforced: true, Path: "/other", Reason: ReasonPathBlocked},
			wantLine: "proto=https host=raw.example.test port=443 path=/other decision=deny reason=path-blocked",
		},
	} {
		t.Run("records "+tc.name, func(t *testing.T) {
			// Arrange
			var jsonBuf bytes.Buffer
			humanBuf := stubSlog(t, time.UTC)
			logger := NewLogger(&jsonBuf)
			logger.now = func() time.Time { return now }

			// Act
			logger.Log(tc.entry)

			// Assert
			var got Entry
			require.NoError(t, json.Unmarshal(jsonBuf.Bytes(), &got))
			want := tc.entry
			want.Time = now
			assert.Equal(t, want, got)
			assert.Equal(t, 1, strings.Count(jsonBuf.String(), "\n"))
			assert.Equal(t, "time=2026-06-17T12:00:00.000Z level=INFO msg=request "+tc.wantLine+"\n", humanBuf.String())
		})
	}

	t.Run("nil file writes no JSON but still logs the request", func(t *testing.T) {
		// Arrange
		buf := stubSlog(t, time.UTC)
		logger := NewLogger(nil)
		logger.now = func() time.Time { return now }

		// Act
		logger.Log(Entry{Protocol: ProtocolHTTPS, Host: "api.example.test", Port: "443", Decision: DecisionAllow, Enforced: true})

		// Assert
		assert.Equal(t, "time=2026-06-17T12:00:00.000Z level=INFO msg=request proto=https host=api.example.test port=443 decision=allow\n", buf.String())
	})
}

func TestDisplayHost(t *testing.T) {
	t.Run("plain host is unchanged", func(t *testing.T) {
		// Act
		host := DisplayHost("api.example.test")

		// Assert
		assert.Equal(t, "api.example.test", host)
	})

	t.Run("non-printable or non-ASCII host is quoted", func(t *testing.T) {
		for _, raw := range []string{"evil\u202e.test", "evil.test\tok", "b\u00fccher.test"} {
			// Act
			host := DisplayHost(raw)

			// Assert
			assert.Equal(t, strconv.QuoteToASCII(raw), host, raw)
		}
	})
}

func TestPruneLogs(t *testing.T) {
	writeFile := func(t *testing.T, dir, name string, age time.Duration) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte("{}\n"), 0o644))
		require.NoError(t, os.Chtimes(path, time.Now().Add(-age), time.Now().Add(-age)))
		return path
	}

	t.Run("removes files older than maxAge and keeps recent ones", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		old := writeFile(t, dir, LogFilePrefix+"old.jsonl", 48*time.Hour)
		recent := writeFile(t, dir, LogFilePrefix+"recent.jsonl", time.Hour)

		// Act
		PruneLogs(dir, 24*time.Hour)

		// Assert
		assert.NoFileExists(t, old)
		assert.FileExists(t, recent)
	})

	for _, name := range []string{LogFilePrefix + "old.txt", "old.jsonl"} {
		t.Run("ignores "+name+" regardless of age", func(t *testing.T) {
			// Arrange
			dir := t.TempDir()
			other := writeFile(t, dir, name, 48*time.Hour)

			// Act
			PruneLogs(dir, 24*time.Hour)

			// Assert
			assert.FileExists(t, other)
		})
	}

	t.Run("maxAge of zero removes every log file", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		old := writeFile(t, dir, LogFilePrefix+"old.jsonl", 48*time.Hour)
		recent := writeFile(t, dir, LogFilePrefix+"recent.jsonl", time.Minute)

		// Act
		PruneLogs(dir, 0)

		// Assert
		assert.NoFileExists(t, old)
		assert.NoFileExists(t, recent)
	})

	t.Run("missing directory is a no-op", func(t *testing.T) {
		// Arrange
		dir := filepath.Join(t.TempDir(), "absent")

		// Act + Assert
		assert.NotPanics(t, func() { PruneLogs(dir, 24*time.Hour) })
	})
}

func TestNewLogHandler(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	l := slog.New(NewLogHandler(&buf, time.FixedZone("", 2*60*60)))
	r := slog.NewRecord(time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC), slog.LevelWarn, "could not remove secret file", 0)

	// Act
	err := l.Handler().Handle(context.Background(), r)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "time=2026-06-17T14:00:00.000+02:00 level=WARN msg=\"could not remove secret file\"\n", buf.String())
}

func Test_httpErrorLog(t *testing.T) {
	// Arrange
	buf := stubSlog(t, time.UTC)

	// Act
	httpErrorLog().Print("http: TLS handshake error")

	// Assert
	assert.Contains(t, buf.String(), `level=ERROR msg="http: TLS handshake error"`)
}

func Test_Entry_refused(t *testing.T) {
	cases := map[string]struct {
		monitor      bool
		wantEnforced bool
	}{
		"enforced":     {false, true},
		"monitor mode": {true, false},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			entry := Entry{Host: "api.example.test", Decision: DecisionAllow}

			// Act
			got := entry.refused(ReasonPathBlocked, c.monitor)

			// Assert
			assert.Equal(t, DecisionDeny, got.Decision)
			assert.Equal(t, c.wantEnforced, got.Enforced)
			assert.Equal(t, ReasonPathBlocked, got.Reason)
		})
	}
}
