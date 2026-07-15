package paper

import "testing"

func TestValidateTemplateOptionRegions(t *testing.T) {
	valid := TemplateLayout{Pages: []TemplatePage{{
		PageNo: 1, Width: 1000, Height: 1400,
		QuestionRegions: []LayoutRegion{{
			ID: "q1-region", QuestionID: "question-1", X: 0.1, Y: 0.1, Width: 0.8, Height: 0.2,
			OptionRegions: []OptionRegion{
				{ID: "a", Label: "A", X: 0.05, Y: 0.2, Width: 0.1, Height: 0.5},
				{ID: "b", Label: "B", X: 0.25, Y: 0.2, Width: 0.1, Height: 0.5},
			},
		}},
	}}}
	if err := ValidateTemplateInput("objective sheet", 1, valid); err != nil {
		t.Fatalf("valid option regions rejected: %v", err)
	}

	duplicate := valid
	duplicate.Pages[0].QuestionRegions[0].OptionRegions[1].Label = "a"
	if err := ValidateTemplateInput("objective sheet", 1, duplicate); err == nil {
		t.Fatal("duplicate option labels must be rejected")
	}

	outOfBounds := valid
	outOfBounds.Pages[0].QuestionRegions[0].OptionRegions = []OptionRegion{{ID: "a", Label: "A", X: 0.95, Y: 0, Width: 0.1, Height: 0.2}}
	if err := ValidateTemplateInput("objective sheet", 1, outOfBounds); err == nil {
		t.Fatal("out of bounds option region must be rejected")
	}
}
