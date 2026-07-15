package paper

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

const (
	maxTemplatePages   = 100
	maxTemplateRegions = 2000
)

var subjectiveQuestionTypes = map[string]bool{
	"formula":      true,
	"short_answer": true,
	"calculation":  true,
	"essay":        true,
	"discussion":   true,
	"coding":       true,
}

func ValidateTemplateInput(name string, pageCount int, layout TemplateLayout) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("template name is required")
	}
	if err := ValidateTemplateOMRProfile(layout.OMRProfile); err != nil {
		return err
	}
	if pageCount < 1 || pageCount > maxTemplatePages || len(layout.Pages) != pageCount {
		return fmt.Errorf("page_count must match 1-%d layout pages", maxTemplatePages)
	}
	pageNos := map[int]bool{}
	questionIDs := map[string]bool{}
	regions := 0
	for _, page := range layout.Pages {
		if page.PageNo < 1 || page.PageNo > pageCount || pageNos[page.PageNo] {
			return errors.New("layout contains an invalid or duplicate page_no")
		}
		pageNos[page.PageNo] = true
		if page.Width < 1 || page.Height < 1 || page.Width > 20000 || page.Height > 20000 {
			return errors.New("page dimensions must be between 1 and 20000 pixels")
		}
		groups := [][]LayoutRegion{page.RegistrationMarks, page.IdentityRegions, page.QuestionRegions}
		for groupIndex, group := range groups {
			regions += len(group)
			for _, region := range group {
				if err := validateRegion(region); err != nil {
					return err
				}
				if groupIndex == 2 {
					if strings.TrimSpace(region.QuestionID) == "" {
						return errors.New("question region must reference question_id")
					}
					if questionIDs[region.QuestionID] {
						return errors.New("question_id may only have one template region")
					}
					questionIDs[region.QuestionID] = true
					labels := map[string]bool{}
					regions += len(region.OptionRegions)
					for _, option := range region.OptionRegions {
						label := strings.ToUpper(strings.TrimSpace(option.Label))
						if label == "" || len(label) > 16 || labels[label] {
							return errors.New("option regions require unique labels up to 16 characters")
						}
						labels[label] = true
						if err := validateOptionRegion(option); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	if regions > maxTemplateRegions {
		return fmt.Errorf("layout exceeds %d regions", maxTemplateRegions)
	}
	return nil
}

func validateOptionRegion(region OptionRegion) error {
	return validateRegion(LayoutRegion{X: region.X, Y: region.Y, Width: region.Width, Height: region.Height})
}

func validateRegion(region LayoutRegion) error {
	values := []float64{region.X, region.Y, region.Width, region.Height}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return errors.New("region coordinates must be finite numbers")
		}
	}
	if region.X < 0 || region.Y < 0 || region.Width <= 0 || region.Height <= 0 || region.X+region.Width > 1.000001 || region.Y+region.Height > 1.000001 {
		return errors.New("region coordinates must stay inside the normalized page")
	}
	return nil
}

func stableContentHash(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func buildReadiness(total float64, classCount int, studentCount int, papers []Paper, questions []Question, templates []AnswerSheetTemplate) ReadinessResult {
	checks := []ReadinessCheck{}
	add := func(code string, label string, passed bool, message string, section string) {
		checks = append(checks, ReadinessCheck{Code: code, Label: label, Passed: passed, Severity: "blocker", Message: message, Section: section})
	}
	add("student_scope", "学生范围", classCount > 0 && studentCount > 0, fmt.Sprintf("已关联 %d 个班级、%d 名在读学生", classCount, studentCount), "students")
	add("paper_file", "试卷文件", len(papers) > 0, fmt.Sprintf("已登记 %d 个试卷版本", len(papers)), "paper")
	add("questions", "题目结构", len(questions) > 0, fmt.Sprintf("已配置 %d 道题", len(questions)), "questions")

	scoreTotal := 0.0
	answersOK := len(questions) > 0
	rubricsOK := len(questions) > 0
	for _, question := range questions {
		scoreTotal += question.Score
		if question.AnswerKey == nil || emptyAnswer(question.AnswerKey.StandardAnswer) {
			answersOK = false
		}
		if subjectiveQuestionTypes[question.QuestionType] && (question.Rubric == nil || question.Rubric.Status != "locked" || !scoreEqual(question.Rubric.MaxScore, question.Score)) {
			rubricsOK = false
		}
	}
	add("total_score", "题目总分", len(questions) > 0 && scoreEqual(scoreTotal, total), fmt.Sprintf("题目合计 %.2f 分，考试总分 %.2f 分", scoreTotal, total), "questions")
	add("answer_keys", "标准答案", answersOK, "每道题都需要标准答案", "questions")
	add("rubrics", "主观题 Rubric", rubricsOK, "所有主观题需要锁定且分值匹配的 Rubric", "questions")

	var locked *AnswerSheetTemplate
	for index := range templates {
		if templates[index].Status == "locked" && (locked == nil || templates[index].VersionNo > locked.VersionNo) {
			locked = &templates[index]
		}
	}
	add("locked_template", "答卷模板", locked != nil, "需要一个已锁定的答卷模板", "template")
	coverageOK := locked != nil && len(questions) > 0
	if locked != nil {
		covered := map[string]bool{}
		for _, page := range locked.Layout.Pages {
			for _, region := range page.QuestionRegions {
				covered[region.QuestionID] = true
			}
		}
		for _, question := range questions {
			if !covered[question.ID] {
				coverageOK = false
			}
		}
	}
	add("template_coverage", "题目区域", coverageOK, "模板必须为每道题配置一个合法答题区域", "template")

	ready := true
	for _, check := range checks {
		if !check.Passed {
			ready = false
		}
	}
	configuration := struct {
		Total        float64
		ClassCount   int
		StudentCount int
		Papers       []Paper
		Questions    []Question
		Templates    []AnswerSheetTemplate
	}{total, classCount, studentCount, papers, questions, templates}
	return ReadinessResult{Ready: ready, ConfigurationHash: stableContentHash(configuration), Checks: checks}
}

func emptyAnswer(value any) bool {
	if value == nil {
		return true
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	return false
}
