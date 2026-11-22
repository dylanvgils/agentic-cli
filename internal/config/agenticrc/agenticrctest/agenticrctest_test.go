package agenticrctest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type Limits struct {
	CPUs string `toml:"cpus"`
}

type item struct {
	Host  string   `toml:"host"`
	Paths []string `toml:"paths"`
}

type proxy struct {
	Enabled *bool  `toml:"enabled"`
	Items   []item `toml:"items"`
}

type sample struct {
	Name string `toml:"name"`
	Limits
	Proxy proxy               `toml:"proxy"`
	Args  map[string][]string `toml:"args"`
}

func TestKeys(t *testing.T) {
	// Act
	keys := Keys[sample]()

	// Assert
	assert.Equal(t, []string{
		"name",
		"cpus",
		"proxy.enabled",
		"proxy.items",
		"args",
	}, keys)
}

func TestAllKeys(t *testing.T) {
	// Act
	keys := AllKeys[sample]()

	// Assert
	assert.Equal(t, []string{
		"name",
		"cpus",
		"proxy.enabled",
		"proxy.items",
		"proxy.items.host",
		"proxy.items.paths",
		"args",
	}, keys)
}

func TestTables(t *testing.T) {
	// Act
	tables := Tables[sample]()

	// Assert
	assert.Equal(t, []string{"proxy"}, tables)
}

func TestLeaves(t *testing.T) {
	// Arrange
	cfg := sample{Name: "example"}

	// Act
	leaves := Leaves(&cfg)

	// Assert
	assert.Len(t, leaves, 5)
	assert.Equal(t, "example", leaves["name"].String())
}

func TestFilled(t *testing.T) {
	// Arrange
	enabled := true

	// Act
	cfg := Filled[sample]("v")

	// Assert
	assert.Equal(t, sample{
		Name:   "v",
		Limits: Limits{CPUs: "v"},
		Proxy: proxy{
			Enabled: &enabled,
			Items:   []item{{Host: "v", Paths: []string{"v"}}},
		},
		Args: map[string][]string{MapKey: {"v"}},
	}, cfg)
}
