package storage

import "testing"

func TestMigrationCommentDoesNotHideFollowingStatement(t *testing.T) {
	statements := splitSQL("-- Explain a data migration\nDELETE FROM example WHERE obsolete=TRUE;\n-- Index after cleanup\nCREATE UNIQUE INDEX example_key ON example(key);\n-- Comment only")
	if len(statements) != 2 || statements[0] != "DELETE FROM example WHERE obsolete=TRUE" || statements[1] != "CREATE UNIQUE INDEX example_key ON example(key)" {
		t.Fatalf("lost migration statements: %q", statements)
	}
}
