// Package migrations is the changelog: every versioned change ever shipped
// to $AGENTIC_HOME's on-disk layout, oldest first, each a plain exported
// function of the form func(toolHome string) error. internal/migrate wires
// these into an ordered internal/migrate.Migration list and applies them;
// this package only defines what each one does.
package migrations
