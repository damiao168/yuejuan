package mathunderstanding

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMixedPerceptionMigrationRoutesOnlyUnstartedMathTasks(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000145_story_math09a_mixed_perception.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, fragment := range []string{
		"THEN 'mixed'",
		"'math-understanding-task-v2'",
		"status='queued'",
		"payload->>'region_kind'='formula'",
		"archetype_code = 'structured_steps'",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("mixed perception migration must contain %q", fragment)
		}
	}
	if strings.Contains(sql, "status IN ('queued','leased','running')") {
		t.Fatal("migration must not mutate leased or running immutable task input")
	}
}
