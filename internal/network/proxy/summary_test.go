package proxy

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Summary_Flagged(t *testing.T) {
	cases := map[string]struct {
		summary Summary
		want    bool
	}{
		"only attempts": {Summary{Attempts: 3}, false},
		"denied":        {Summary{Denied: HostCount{Count: 1}}, true},
		"paths":         {Summary{Paths: HostCount{Count: 1}}, true},
		"refused":       {Summary{Refused: HostCount{Count: 1}}, true},
		"mismatched":    {Summary{Mismatched: HostCount{Count: 1}}, true},
		"missed":        {Summary{Missed: HostCount{Count: 1}}, true},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			got := c.summary.Flagged()

			// Assert
			assert.Equal(t, c.want, got)
		})
	}
}

func Test_Entry_repeatsAttempt(t *testing.T) {
	cases := map[string]struct {
		entry Entry
		want  bool
	}{
		"intercepted tunnel":             {Entry{Intercepted: true}, false},
		"failed request inside a tunnel": {Entry{Intercepted: true, Reason: ReasonUpstreamFailed}, true},
		"missed placeholder":             {Entry{Intercepted: true, Reason: ReasonNoPlaceholder}, true},
		"failed plain tunnel":            {Entry{Reason: ReasonUpstreamFailed}, false},
		"monitor mode path verdict":      {Entry{Reason: ReasonPathBlocked}, true},
		"enforced path block":            {Entry{Reason: ReasonPathBlocked, Enforced: true}, false},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			got := c.entry.repeatsAttempt()

			// Assert
			assert.Equal(t, c.want, got)
		})
	}
}

func TestReadSummary(t *testing.T) {
	t.Run("collects unique denied hosts in first-seen order and their total", func(t *testing.T) {
		// Arrange
		path := writeAccessLog(t,
			`{"host":"api.anthropic.com","decision":"allow"}`,
			`{"host":"evil.com","decision":"deny"}`,
			`{"host":"evil.com","decision":"deny"}`,
			`{"host":"tracker.net","decision":"deny"}`,
		)

		// Act
		summary := ReadSummary(path)

		// Assert
		assert.Equal(t, []string{"evil.com", "tracker.net"}, summary.Denied.Hosts)
		assert.Equal(t, 3, summary.Denied.Count)
		assert.Equal(t, 4, summary.Attempts)
	})

	t.Run("sorts each entry into one kind", func(t *testing.T) {
		// Arrange
		path := writeAccessLog(t,
			`{"host":"down.example.test","decision":"deny","reason":"upstream-failed"}`,
			`{"host":"local.example.test","decision":"deny","reason":"blocked-address"}`,
			`{"host":"cdn.example.test","decision":"deny","reason":"sni-mismatch"}`,
			`{"host":"api.example.test","decision":"deny","intercepted":true,"reason":"no-placeholder"}`,
			`{"host":"raw.example.test","decision":"deny","intercepted":true,"path":"/other","reason":"path-blocked"}`,
		)

		// Act
		summary := ReadSummary(path)

		// Assert
		assert.Equal(t, []string{"down.example.test"}, summary.Denied.Hosts, "a denied host whose upstream failed in monitor mode still counts")
		assert.Equal(t, []string{"raw.example.test/other"}, summary.Paths.Hosts)
		assert.Equal(t, []string{"local.example.test"}, summary.Refused.Hosts)
		assert.Equal(t, []string{"cdn.example.test"}, summary.Mismatched.Hosts)
		assert.Equal(t, []string{"api.example.test"}, summary.Missed.Hosts)
	})

	t.Run("counts a tunnel with failed requests as one attempt", func(t *testing.T) {
		// Arrange
		path := writeAccessLog(t,
			`{"host":"api.example.test","decision":"allow","intercepted":true}`,
			`{"host":"api.example.test","decision":"allow","intercepted":true,"reason":"upstream-failed"}`,
			`{"host":"api.example.test","decision":"allow","intercepted":true,"reason":"no-placeholder"}`,
		)

		// Act
		summary := ReadSummary(path)

		// Assert
		assert.Equal(t, 1, summary.Attempts)
	})

	t.Run("hosts are made safe to print", func(t *testing.T) {
		// Arrange
		path := writeAccessLog(t, `{"host":"evil\u202e.test","decision":"deny"}`)

		// Act
		summary := ReadSummary(path)

		// Assert
		assert.Equal(t, []string{`"evil\u202e.test"`}, summary.Denied.Hosts)
	})

	t.Run("skips lines it can't parse", func(t *testing.T) {
		// Arrange
		path := writeAccessLog(t, `not json`, `{"host":"evil.com","decision":"deny"}`)

		// Act
		summary := ReadSummary(path)

		// Assert
		assert.Equal(t, 1, summary.Denied.Count)
		assert.Equal(t, 1, summary.Attempts)
	})

	t.Run("missing log yields nothing", func(t *testing.T) {
		// Act
		summary := ReadSummary(filepath.Join(t.TempDir(), "absent.jsonl"))

		// Assert
		assert.Equal(t, Summary{}, summary)
	})
}
