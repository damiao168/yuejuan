package paper

import "math"

var validQuestionTypes = map[string]bool{
	"single_choice":   true,
	"multiple_choice": true,
	"true_false":      true,
	"fill_blank":      true,
	"numeric":         true,
	"formula":         true,
	"short_answer":    true,
	"calculation":     true,
	"essay":           true,
	"discussion":      true,
	"coding":          true,
}

var validRubricStatuses = map[string]bool{
	"draft":          true,
	"pending_review": true,
	"approved":       true,
	"locked":         true,
}

func IsValidQuestionType(kind string) bool {
	return validQuestionTypes[kind]
}

func IsValidRubricStatus(status string) bool {
	return validRubricStatuses[status]
}

func SumRubricPoints(points []RubricPoint) float64 {
	total := 0.0
	for _, point := range points {
		total += point.Score
	}
	return total
}

func scoreEqual(a float64, b float64) bool {
	return math.Abs(a-b) < 0.0001
}
