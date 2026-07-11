package exam

var validStatuses = map[string]bool{
	"draft":      true,
	"configured": true,
	"ready":      true,
	"collecting": true,
	"grading":    true,
	"reviewing":  true,
	"finalized":  true,
	"published":  true,
	"archived":   true,
}

var validGradingModes = map[string]bool{
	"auto_objective_only":   true,
	"ai_assisted":           true,
	"human_review_required": true,
	"double_mark":           true,
	"blind_double_mark":     true,
}

var allowedTransitions = map[string]map[string]bool{
	"draft": {
		"configured": true,
		"archived":   true,
	},
	"configured": {
		"archived": true,
	},
	"ready": {
		"configured": true,
		"archived":   true,
	},
	"collecting": {
		"grading":  true,
		"archived": true,
	},
	"grading": {
		"reviewing": true,
		"archived":  true,
	},
	"reviewing": {
		"finalized": true,
		"archived":  true,
	},
	"finalized": {
		"published": true,
		"archived":  true,
	},
	"published": {
		"archived": true,
	},
}

func IsValidStatus(status string) bool {
	return validStatuses[status]
}

func IsValidGradingMode(mode string) bool {
	return validGradingModes[mode]
}

func CanTransition(from string, to string) bool {
	if from == to {
		return true
	}
	return allowedTransitions[from][to]
}

func IsCoreLocked(status string) bool {
	return status == "published" || status == "archived"
}
