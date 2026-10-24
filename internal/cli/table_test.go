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
	t.Run("prints a column per tagged field, aligned under its header", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		rows := []testRow{
			{
				tool:      "claude",
				namespace: "agentic",
				note:      "hidden",
			},
			{
				tool:      "opencode",
				namespace: "work",
			},
		}

		// Act
		err := writeTable(&buf, rows)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "TOOL      NAMESPACE\nclaude    agentic\nopencode  work\n", buf.String())
	})

	t.Run("dashes empty cells", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer

		// Act
		err := writeTable(&buf, []testRow{{tool: "claude"}})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "TOOL    NAMESPACE\nclaude  -\n", buf.String())
	})

	t.Run("prints only the header without rows", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer

		// Act
		err := writeTable[testRow](&buf, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "TOOL  NAMESPACE\n", buf.String())
	})
}

func Test_writeFields(t *testing.T) {
	t.Run("aligns values after the longest key", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		detail := testDetail{
			tool:      "claude",
			namespace: "agentic",
		}

		// Act
		err := writeFields(&buf, "  ", detail)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "  tool:       claude\n  namespace:  agentic\n", buf.String())
	})

	t.Run("skips empty values and untagged fields", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		detail := testDetail{
			tool: "claude",
			note: "hidden",
		}

		// Act
		err := writeFields(&buf, "", detail)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "tool:  claude\n", buf.String())
	})
}
