package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRow is a writeTable row with an untagged field, which gets no column.
type testRow struct {
	tool      string `table:"TOOL"`
	namespace string `table:"NAMESPACE"`
	note      string
}

// testDetail is a writeFields value with an untagged field, which gets no line.
type testDetail struct {
	tool      string `field:"tool"`
	namespace string `field:"namespace"`
	note      string
}

func Test_writeTable(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []testRow
		want string
	}{
		{
			name: "prints a column per tagged field, aligned under its header",
			rows: []testRow{
				{
					tool:      "claude",
					namespace: "agentic",
					note:      "hidden",
				},
				{
					tool:      "opencode",
					namespace: "work",
				},
			},
			want: "TOOL      NAMESPACE\nclaude    agentic\nopencode  work\n",
		},
		{
			name: "dashes empty cells",
			rows: []testRow{{tool: "claude"}},
			want: "TOOL    NAMESPACE\nclaude  -\n",
		},
		{
			name: "prints only the header without rows",
			want: "TOOL  NAMESPACE\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			var buf bytes.Buffer

			// Act
			err := writeTable(&buf, tc.rows)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.want, buf.String())
		})
	}
}

func Test_writeFields(t *testing.T) {
	for _, tc := range []struct {
		name   string
		prefix string
		detail testDetail
		want   string
	}{
		{
			name:   "aligns values after the longest key",
			prefix: "  ",
			detail: testDetail{
				tool:      "claude",
				namespace: "agentic",
			},
			want: "  tool:       claude\n  namespace:  agentic\n",
		},
		{
			name: "skips empty values and untagged fields",
			detail: testDetail{
				tool: "claude",
				note: "hidden",
			},
			want: "tool:  claude\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			var buf bytes.Buffer

			// Act
			err := writeFields(&buf, tc.prefix, tc.detail)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.want, buf.String())
		})
	}
}
