package capture

import "testing"

func TestValidateCorrectionPoints(t *testing.T) {
	valid := []NormalizedPoint{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
	if err := validateCorrectionPoints(valid, valid); err != nil {
		t.Fatal(err)
	}
	invalid := [][]NormalizedPoint{
		{{0, 0}, {1, 1}, {1, 0}, {0, 1}},
		{{0, 0}, {0, 0}, {1, 1}, {0, 1}},
		{{-0.1, 0}, {1, 0}, {1, 1}, {0, 1}},
		{{0, 0}, {.1, 0}, {.1, .1}, {0, .1}},
	}
	for _, points := range invalid {
		if err := validateCorrectionPoints(points, valid); err == nil {
			t.Fatalf("expected invalid points: %#v", points)
		}
	}
}
