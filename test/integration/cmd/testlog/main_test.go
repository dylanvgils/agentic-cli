//go:build integration

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	passingRun = `{"Action":"run","Package":"example.test/egress","Test":"TestA"}
{"Action":"output","Package":"example.test/egress","Test":"TestA","Output":"=== RUN   TestA\n"}
{"Action":"output","Package":"example.test/egress","Test":"TestA","Output":"a log line\n"}
{"Action":"run","Package":"example.test/egress","Test":"TestA/sub"}
{"Action":"pass","Package":"example.test/egress","Test":"TestA/sub","Elapsed":0.5}
{"Action":"pass","Package":"example.test/egress","Test":"TestA","Elapsed":1.5}
{"Action":"skip","Package":"example.test/egress","Test":"TestB","Elapsed":0}
{"Action":"pass","Package":"example.test/egress","Elapsed":2}
`
	noTestFilesRun = `{"Action":"start","Package":"example.test/platform"}
{"Action":"output","Package":"example.test/platform","Output":"?   \texample.test/platform\t[no test files]\n"}
{"Action":"skip","Package":"example.test/platform","Elapsed":0}
`
	failingRun = `{"Action":"output","Package":"example.test/proxy","Test":"TestC","Output":"=== RUN   TestC\n"}
{"Action":"run","Package":"example.test/proxy","Test":"TestC/sub"}
{"Action":"output","Package":"example.test/proxy","Test":"TestC/sub","Output":"sub went wrong\n"}
{"Action":"fail","Package":"example.test/proxy","Test":"TestC/sub","Elapsed":0.2}
{"Action":"output","Package":"example.test/proxy","Test":"TestC","Output":"parent noise\n"}
{"Action":"fail","Package":"example.test/proxy","Test":"TestC","Elapsed":0.3}
{"Action":"fail","Package":"example.test/proxy","Elapsed":1}
`
)

func Test_run(t *testing.T) {
	t.Run("prints each test with its subtests indented below it, and package results", func(t *testing.T) {
		// Arrange
		var out bytes.Buffer

		// Act
		ok := run(strings.NewReader(passingRun), &out, options{})

		// Assert
		assert.True(t, ok)
		assert.Contains(t, out.String(), "PASS egress TestA (1.5s)\n  PASS sub (0.5s)\nSKIP egress TestB (0.0s)\n")
		assert.Contains(t, out.String(), "SKIP egress TestB (0.0s)\n")
		assert.Contains(t, out.String(), "PASS egress (2.0s)\n")
		assert.NotContains(t, out.String(), "a log line")
		assert.Contains(t, out.String(), "PASS egress (2.0s)\n\n\n=== Slowest\n")
		assert.Contains(t, out.String(), "\n\nDONE 1 packages, 3 tests, 1 skipped, 0 failed")
	})

	t.Run("prints the output of the deepest failures at the end", func(t *testing.T) {
		// Arrange
		var out bytes.Buffer

		// Act
		ok := run(strings.NewReader(failingRun), &out, options{})

		// Assert
		assert.False(t, ok)
		assert.Contains(t, out.String(), "FAIL proxy TestC (0.3s)\n  FAIL sub (0.2s)\n")
		assert.Contains(t, out.String(), "--- FAIL proxy TestC/sub\nsub went wrong\n")
		assert.NotContains(t, out.String(), "--- FAIL proxy TestC\n")
		assert.Contains(t, out.String(), "--- FAIL proxy\n")
		assert.NotContains(t, out.String(), "=== RUN")
	})

	t.Run("folds each package log into a group", func(t *testing.T) {
		// Arrange
		var out bytes.Buffer

		// Act
		run(strings.NewReader(passingRun), &out, options{groups: true})

		// Assert
		assert.Contains(t, out.String(), "::group::egress log\n=== RUN   TestA\na log line\n::endgroup::\n")
	})

	t.Run("passes lines that are not events through", func(t *testing.T) {
		// Arrange
		var out bytes.Buffer

		// Act
		ok := run(strings.NewReader("go: no packages to test\n"), &out, options{})

		// Assert
		assert.False(t, ok)
		assert.Contains(t, out.String(), "go: no packages to test\n")
	})

	t.Run("hides packages without test files", func(t *testing.T) {
		// Arrange
		in := noTestFilesRun + passingRun
		var out bytes.Buffer

		// Act
		ok := run(strings.NewReader(in), &out, options{groups: true})

		// Assert
		assert.True(t, ok)
		assert.NotContains(t, out.String(), "platform")
		assert.Contains(t, out.String(), "DONE 1 packages")
	})

	t.Run("lists the slowest tests without subtests", func(t *testing.T) {
		// Arrange
		var out bytes.Buffer

		// Act
		run(strings.NewReader(passingRun), &out, options{})

		// Assert
		assert.Contains(t, out.String(), "=== Slowest\n    0.5s egress TestA/sub\n    0.0s egress TestB\n")
	})

	t.Run("colors results when color is on", func(t *testing.T) {
		// Arrange
		var out bytes.Buffer

		// Act
		run(strings.NewReader(passingRun), &out, options{color: true})

		// Assert
		assert.Contains(t, out.String(), "\x1b[32mPASS\x1b[0m egress TestA (1.5s)\n")
		assert.Contains(t, out.String(), "\x1b[33mSKIP\x1b[0m egress TestB (0.0s)\n")
		assert.Contains(t, out.String(), "\x1b[32mDONE 1 packages")
	})
}
