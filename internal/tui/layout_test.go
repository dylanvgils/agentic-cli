package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeLayout(t *testing.T) {
	t.Run("splits a typical terminal into a status box, left list panels, and detail column", func(t *testing.T) {
		// Act
		l := computeLayout(100, 40)

		// Assert
		assert.Equal(t, panelLayout{boxWidth: 38, boxHeight: 4, tableWidth: 36, tableHeight: 3}, l.status)
		assert.Equal(t, panelLayout{boxWidth: 38, boxHeight: 8, tableWidth: 36, tableHeight: 7}, l.left[0])
		assert.Equal(t, panelLayout{boxWidth: 38, boxHeight: 8, tableWidth: 36, tableHeight: 7}, l.left[1])
		assert.Equal(t, panelLayout{boxWidth: 38, boxHeight: 10, tableWidth: 36, tableHeight: 9}, l.left[2])
		assert.Equal(t, panelLayout{boxWidth: 58, boxHeight: 36, tableWidth: 56, tableHeight: 36}, l.detail)
	})

	t.Run("remainder rows from an uneven body height go to the volumes panel", func(t *testing.T) {
		// Act
		l := computeLayout(100, 42)

		// Assert - body=40, listBody=34, base=11, extra=1, so the last panel's boxHeight is 10 vs. 9.
		assert.Equal(t, 9, l.left[0].boxHeight)
		assert.Equal(t, 9, l.left[1].boxHeight)
		assert.Equal(t, 10, l.left[2].boxHeight)
	})

	t.Run("dimensions never go below the minimum floor on a tiny terminal", func(t *testing.T) {
		// Act
		l := computeLayout(1, 1)

		// Assert
		for _, p := range l.left {
			assert.Equal(t, 1, p.boxWidth)
			assert.Equal(t, 1, p.boxHeight)
			assert.Equal(t, 1, p.tableWidth)
			assert.Equal(t, 1, p.tableHeight)
		}
		assert.Equal(t, 1, l.detail.boxWidth)
		assert.Equal(t, 1, l.detail.boxHeight)

		// The status box has a fixed content height, so unlike the other
		// panels it doesn't floor to 1 on a tiny terminal.
		assert.Equal(t, 1, l.status.boxWidth)
		assert.Equal(t, 4, l.status.boxHeight)
	})
}
