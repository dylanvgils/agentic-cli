package agenticrc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config/agenticrc/agenticrctest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchema_matchesConfig(t *testing.T) {
	// Arrange
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "agenticrc.schema.json"))
	require.NoError(t, err)
	var schema map[string]any
	require.NoError(t, json.Unmarshal(data, &schema))
	want := append(agenticrctest.AllKeys[Config](), agenticrctest.Tables[Config]()...)

	// Act
	var keys []string
	for _, key := range schemaKeys(schema, schema, "") {
		// Its properties are layer names, checked against knownExtras in image/tools
		if !strings.HasPrefix(key, "build.versions.") {
			keys = append(keys, key)
		}
	}

	// Assert
	assert.ElementsMatch(t, want, keys, "every .agenticrc.toml key and table needs a property in agenticrc.schema.json")
}

// schemaKeys returns the dotted path of every property below node, following $ref, items and oneOf/anyOf/allOf.
func schemaKeys(root, node map[string]any, prefix string) []string {
	node = resolveRef(root, node)

	var subs []map[string]any
	if items, ok := node["items"].(map[string]any); ok {
		subs = append(subs, items)
	}
	for _, keyword := range []string{"oneOf", "anyOf", "allOf"} {
		branches, _ := node[keyword].([]any)
		for _, branch := range branches {
			if sub, ok := branch.(map[string]any); ok {
				subs = append(subs, sub)
			}
		}
	}

	var keys []string
	for _, sub := range subs {
		keys = append(keys, schemaKeys(root, sub, prefix)...)
	}
	properties, _ := node["properties"].(map[string]any)
	for name, property := range properties {
		keys = append(keys, prefix+name)
		if sub, ok := property.(map[string]any); ok {
			keys = append(keys, schemaKeys(root, sub, prefix+name+".")...)
		}
	}

	// A property listed in several branches is still one key
	slices.Sort(keys)
	return slices.Compact(keys)
}

// resolveRef returns the root $defs entry node's "#/$defs/<name>" $ref points to, or node when it has none.
func resolveRef(root, node map[string]any) map[string]any {
	ref, ok := node["$ref"].(string)
	if !ok {
		return node
	}

	defs, _ := root["$defs"].(map[string]any)
	def, _ := defs[strings.TrimPrefix(ref, "#/$defs/")].(map[string]any)
	return def
}
