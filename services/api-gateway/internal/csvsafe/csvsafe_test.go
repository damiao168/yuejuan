package csvsafe

import "testing"

func TestCellNeutralizesSpreadsheetFormulaPrefixes(t *testing.T) {
	for _, value := range []string{`=HYPERLINK("https://example.invalid")`, "+CMD", "-1+1", "@SUM(A1:A2)", "\t=1+1", "\r=1+1", "  =1+1"} {
		if got := Cell(value); len(got) == 0 || got[0] != '\'' {
			t.Fatalf("dangerous cell %q was not neutralized: %q", value, got)
		}
	}
	for _, value := range []string{"张三", "SIM-001", "42", ""} {
		if got := Cell(value); got != value {
			t.Fatalf("safe cell %q changed to %q", value, got)
		}
	}
}
