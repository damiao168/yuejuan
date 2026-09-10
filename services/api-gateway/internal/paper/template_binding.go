package paper

import "strings"

func normalizeBindingMode(mode string) (string, bool) {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		mode = "locked_with_guard"
	}
	switch mode {
	case "locked_with_guard":
		return mode, true
	default:
		return "", false
	}
}
