// Package csvsafe neutralizes spreadsheet formulas in exported, user-derived
// CSV cells. CSV quoting alone does not stop spreadsheet formula evaluation.
package csvsafe

import "strings"

func Cell(value string) string {
	trimmed := strings.TrimLeft(value, " \u00a0")
	if trimmed == "" {
		return value
	}
	switch trimmed[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + value
	default:
		return value
	}
}

func Row(values []string) []string {
	out := make([]string, len(values))
	for index, value := range values {
		out[index] = Cell(value)
	}
	return out
}
