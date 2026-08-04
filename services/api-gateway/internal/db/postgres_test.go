package db

import "testing"

func TestSQLOperationDoesNotExposeStatementText(t *testing.T) {
	tests := map[string]string{
		"SELECT * FROM student WHERE name = 'secret'": "select",
		"\nUPDATE exam SET status = $1":               "update",
		"student private answer":                      "other",
		"":                                            "unknown",
	}
	for statement, expected := range tests {
		if actual := sqlOperation(statement); actual != expected {
			t.Fatalf("sqlOperation(%q)=%q, want %q", statement, actual, expected)
		}
	}
}
