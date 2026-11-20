package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_versionsForLayer(t *testing.T) {
	v := Versions{Node: "22", Java: "25", Dotnet: "10", Go: "1.26.3"}

	for _, tc := range []struct {
		name  string
		layer string
		want  string
	}{
		{
			name:  "node returns node field",
			layer: "node",
			want:  "22",
		},
		{
			name:  "java returns java field",
			layer: "java",
			want:  "25",
		},
		{
			name:  "dotnet returns dotnet field",
			layer: "dotnet",
			want:  "10",
		},
		{
			name:  "go returns go field",
			layer: "go",
			want:  "1.26.3",
		},
		{
			name:  "unknown name returns empty string",
			layer: "ruby",
			want:  "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := v.ForLayer(tc.layer)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func Test_mustDecode(t *testing.T) {
	t.Run("decodes valid JSON", func(t *testing.T) {
		// Act
		got := mustDecode[Versions]([]byte(`{"node":"22"}`), "versions.json")

		// Assert
		assert.Equal(t, "22", got.Node)
	})

	t.Run("panics on malformed JSON", func(t *testing.T) {
		// Act + Assert
		assert.PanicsWithValue(t, "tools: parse embedded versions.json: unexpected end of JSON input", func() {
			mustDecode[Versions]([]byte(`{`), "versions.json")
		})
	})
}
