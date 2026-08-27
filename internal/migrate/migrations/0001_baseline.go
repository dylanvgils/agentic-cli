package migrations

// Baseline establishes schema version 1. It performs no changes - it exists
// to seed the changelog and exercise the migration mechanism; there is no
// pre-existing on-disk inconsistency to fix as of this schema version.
func Baseline(_ string) error {
	return nil
}
