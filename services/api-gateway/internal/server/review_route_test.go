package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/config"
	"edugrade-enterprise/services/api-gateway/internal/exam"
	"edugrade-enterprise/services/api-gateway/internal/files"
	"edugrade-enterprise/services/api-gateway/internal/logger"
	ocrpkg "edugrade-enterprise/services/api-gateway/internal/ocr"
	"edugrade-enterprise/services/api-gateway/internal/org"
	"edugrade-enterprise/services/api-gateway/internal/paper"
	"edugrade-enterprise/services/api-gateway/internal/review"
	"edugrade-enterprise/services/api-gateway/internal/segment"
	"edugrade-enterprise/services/api-gateway/internal/submission"
)

const reviewTenantID = "00000000-0000-0000-0000-000000000002"
const reviewManagerID = "00000000-0000-0000-0000-000000000601"
const reviewGraderID = "00000000-0000-0000-0000-000000000602"
const reviewSecondGraderID = "00000000-0000-0000-0000-000000000603"
const reviewArbitratorID = "00000000-0000-0000-0000-000000000604"
const reviewOtherArbitratorID = "00000000-0000-0000-0000-000000000605"

func TestReviewTaskRoutesCreateAssignSubmitAndAudit(t *testing.T) {
	authStore := reviewAuthStore(t, []string{"review:manage"})
	reviewStore := review.NewMemoryStore()
	reviewStore.AddContext(reviewTenantID, "segment-1", reviewRouteContext())
	router := reviewRouter(authStore, reviewStore)
	managerToken := reviewLogin(t, router, "review_manager")
	graderToken := reviewLogin(t, router, "review_grader")

	req := reviewAuthedRequest(http.MethodPost, "/api/v1/review-tasks", `{"answer_segment_id":"segment-1","source":"evidence_verification_failed","priority":3}`, managerToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"anonymous_code":"ANON-001"`) || strings.Contains(rec.Body.String(), "student_name") {
		t.Fatalf("create review task expected anonymous task, got %d %s", rec.Code, rec.Body.String())
	}
	taskID := decodeReviewTaskID(t, rec.Body.Bytes())

	req = reviewAuthedRequest(http.MethodGet, "/api/v1/review-tasks", "", managerToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"anonymous_code":"ANON-001"`) {
		t.Fatalf("list review tasks expected anonymous task, got %d %s", rec.Code, rec.Body.String())
	}

	req = reviewAuthedRequest(http.MethodGet, "/api/v1/review-tasks/"+taskID, "", managerToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+taskID+`"`) {
		t.Fatalf("get review task expected task detail, got %d %s", rec.Code, rec.Body.String())
	}

	req = reviewAuthedRequest(http.MethodPost, "/api/v1/review-tasks/"+taskID+"/assign", `{"assigned_to":"`+reviewGraderID+`"}`, managerToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"assigned"`) {
		t.Fatalf("assign review task expected assigned, got %d %s", rec.Code, rec.Body.String())
	}

	req = reviewAuthedRequest(http.MethodPost, "/api/v1/review-tasks/"+taskID+"/submit", `{"score":4,"rubric_selections":[{"point_id":"p1","score":4}],"comments":"clear","reason":"manual review"}`, graderToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"status":"submitted"`) || !strings.Contains(rec.Body.String(), `"score":4`) {
		t.Fatalf("submit human grade expected submitted grade, got %d %s", rec.Code, rec.Body.String())
	}

	req = reviewAuthedRequest(http.MethodPost, "/api/v1/review-tasks/"+taskID+"/return", `{"reason":"needs second look"}`, managerToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"returned"`) || !strings.Contains(rec.Body.String(), `"return_reason":"needs second look"`) {
		t.Fatalf("return review task expected returned, got %d %s", rec.Code, rec.Body.String())
	}

	reviewAssertAuditAction(t, authStore, "review.task_created")
	reviewAssertAuditAction(t, authStore, "review.task_assigned")
	reviewAssertAuditAction(t, authStore, "review.human_grade_submitted")
	reviewAssertAuditAction(t, authStore, "review.task_returned")
}

func TestReviewTaskRoutesRequirePermission(t *testing.T) {
	authStore := reviewAuthStore(t, []string{"system:read"})
	reviewStore := review.NewMemoryStore()
	reviewStore.AddContext(reviewTenantID, "segment-1", reviewRouteContext())
	router := reviewRouter(authStore, reviewStore)
	token := reviewLogin(t, router, "review_manager")

	req := reviewAuthedRequest(http.MethodPost, "/api/v1/review-tasks", `{"answer_segment_id":"segment-1","source":"manual_sample"}`, token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/review-tasks", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d %s", rec.Code, rec.Body.String())
	}

	req = reviewAuthedRequest(http.MethodGet, "/api/v1/arbitration-tasks", "", token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("arbitration route expected 403, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestReviewWorkPermissionIsScopedToAssignedTasks(t *testing.T) {
	authStore := reviewAuthStoreByUser(t, map[string][]string{
		"review_manager":          {"review:manage"},
		"review_grader":           {"review:work"},
		"review_second_grader":    {"review:work"},
		"review_arbitrator":       {"arbitration:work"},
		"review_other_arbitrator": {"arbitration:work"},
	})
	reviewStore := review.NewMemoryStore()
	reviewStore.AddContext(reviewTenantID, "segment-1", reviewRouteContext())
	reviewStore.AddContext(reviewTenantID, "segment-2", reviewRouteContext())
	router := reviewRouter(authStore, reviewStore)
	managerToken := reviewLogin(t, router, "review_manager")
	graderToken := reviewLogin(t, router, "review_grader")

	firstID := createReviewTaskForRoute(t, router, managerToken, "segment-1", "manual_sample")
	secondID := createReviewTaskForRoute(t, router, managerToken, "segment-2", "score_anomaly")
	assignReviewTaskForRoute(t, router, managerToken, firstID, reviewGraderID)
	assignReviewTaskForRoute(t, router, managerToken, secondID, reviewSecondGraderID)

	req := reviewAuthedRequest(http.MethodGet, "/api/v1/review-tasks", "", graderToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), firstID) || strings.Contains(rec.Body.String(), secondID) {
		t.Fatalf("review worker list should contain only assigned tasks, got %d %s", rec.Code, rec.Body.String())
	}

	req = reviewAuthedRequest(http.MethodGet, "/api/v1/review-tasks/"+firstID, "", graderToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), firstID) {
		t.Fatalf("review worker should get assigned task, got %d %s", rec.Code, rec.Body.String())
	}

	req = reviewAuthedRequest(http.MethodGet, "/api/v1/review-tasks/"+secondID, "", graderToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("review worker should not get another assignee task, got %d %s", rec.Code, rec.Body.String())
	}

	req = reviewAuthedRequest(http.MethodPost, "/api/v1/review-tasks/"+secondID+"/assign", `{"assigned_to":"`+reviewGraderID+`"}`, graderToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("review worker should not assign tasks, got %d %s", rec.Code, rec.Body.String())
	}

	req = reviewAuthedRequest(http.MethodPost, "/api/v1/review-tasks/"+firstID+"/submit", `{"score":4,"rubric_selections":[{"point_id":"p1","score":4}]}`, graderToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("review worker should submit assigned task, got %d %s", rec.Code, rec.Body.String())
	}

	req = reviewAuthedRequest(http.MethodPost, "/api/v1/review-tasks/"+firstID+"/return", `{"reason":"worker cannot return"}`, graderToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("review worker should not return tasks, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestReviewTaskBatchAssignRoute(t *testing.T) {
	authStore := reviewAuthStore(t, []string{"review:manage"})
	reviewStore := review.NewMemoryStore()
	reviewStore.AddContext(reviewTenantID, "segment-1", reviewRouteContext())
	reviewStore.AddContext(reviewTenantID, "segment-2", reviewRouteContext())
	router := reviewRouter(authStore, reviewStore)
	managerToken := reviewLogin(t, router, "review_manager")

	firstID := createReviewTaskForRoute(t, router, managerToken, "segment-1", "manual_sample")
	secondID := createReviewTaskForRoute(t, router, managerToken, "segment-2", "score_anomaly")

	body := `{"task_ids":["` + firstID + `","` + secondID + `"],"assigned_to":"` + reviewGraderID + `"}`
	req := reviewAuthedRequest(http.MethodPost, "/api/v1/review-tasks/batch-assign", body, managerToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || strings.Count(rec.Body.String(), `"status":"assigned"`) != 2 {
		t.Fatalf("batch assign expected two assigned tasks, got %d %s", rec.Code, rec.Body.String())
	}
	reviewAssertAuditAction(t, authStore, "review.tasks_batch_assigned")
}

func TestArbitrationWorkPermissionIsScopedToAssignedTasks(t *testing.T) {
	authStore := reviewAuthStoreByUser(t, map[string][]string{
		"review_manager":          {"review:manage", "arbitration:manage"},
		"review_grader":           {"review:manage"},
		"review_second_grader":    {"review:manage"},
		"review_arbitrator":       {"arbitration:work"},
		"review_other_arbitrator": {"arbitration:work"},
	})
	reviewStore := review.NewMemoryStore()
	reviewStore.AddContext(reviewTenantID, "segment-1", reviewRouteContext())
	router := reviewRouter(authStore, reviewStore)
	managerToken := reviewLogin(t, router, "review_manager")
	firstToken := reviewLogin(t, router, "review_grader")
	secondToken := reviewLogin(t, router, "review_second_grader")
	arbitratorToken := reviewLogin(t, router, "review_arbitrator")
	otherArbitratorToken := reviewLogin(t, router, "review_other_arbitrator")

	req := reviewAuthedRequest(http.MethodPut, "/api/v1/exams/exam-1/double-mark-policy", `{"enabled":true,"threshold":1,"resolution_strategy":"average"}`, managerToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("set double mark policy expected 200, got %d %s", rec.Code, rec.Body.String())
	}

	body := `{"answer_segment_id":"segment-1","first_reviewer_id":"` + reviewGraderID + `","second_reviewer_id":"` + reviewSecondGraderID + `"}`
	req = reviewAuthedRequest(http.MethodPost, "/api/v1/double-mark-sessions", body, managerToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create double mark session expected 201, got %d %s", rec.Code, rec.Body.String())
	}
	session := decodeDoubleMarkSession(t, rec.Body.Bytes())

	req = reviewAuthedRequest(http.MethodPost, "/api/v1/review-tasks/"+session.FirstReviewTaskID+"/submit", `{"score":4,"rubric_selections":[{"point_id":"p1","score":4}]}`, firstToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("submit first mark expected 201, got %d %s", rec.Code, rec.Body.String())
	}

	req = reviewAuthedRequest(http.MethodPost, "/api/v1/review-tasks/"+session.SecondReviewTaskID+"/submit", `{"score":1,"rubric_selections":[{"point_id":"p1","score":1}]}`, secondToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("submit second mark expected 201, got %d %s", rec.Code, rec.Body.String())
	}
	arbitrationID := decodeArbitrationIDFromSubmit(t, rec.Body.Bytes())

	req = reviewAuthedRequest(http.MethodGet, "/api/v1/arbitration-tasks/"+arbitrationID, "", arbitratorToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("unassigned arbitration worker should not get task, got %d %s", rec.Code, rec.Body.String())
	}

	req = reviewAuthedRequest(http.MethodPost, "/api/v1/arbitration-tasks/"+arbitrationID+"/assign", `{"assigned_to":"`+reviewArbitratorID+`"}`, managerToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("assign arbitration expected 200, got %d %s", rec.Code, rec.Body.String())
	}

	req = reviewAuthedRequest(http.MethodGet, "/api/v1/arbitration-tasks/"+arbitrationID, "", arbitratorToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), arbitrationID) {
		t.Fatalf("assigned arbitration worker should get task, got %d %s", rec.Code, rec.Body.String())
	}

	req = reviewAuthedRequest(http.MethodGet, "/api/v1/arbitration-tasks/"+arbitrationID, "", otherArbitratorToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("other arbitration worker should not get task, got %d %s", rec.Code, rec.Body.String())
	}

	req = reviewAuthedRequest(http.MethodPost, "/api/v1/arbitration-tasks/"+arbitrationID+"/submit", `{"final_score":3,"reason":"not assigned","student_feedback":"Final score after arbitration."}`, otherArbitratorToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("other arbitration worker should not submit task, got %d %s", rec.Code, rec.Body.String())
	}

	req = reviewAuthedRequest(http.MethodPost, "/api/v1/arbitration-tasks/"+arbitrationID+"/submit", `{"final_score":3,"reason":"rubric evidence supports middle score","student_feedback":"Final score after arbitration."}`, arbitratorToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("assigned arbitration worker should submit task, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestDoubleMarkAndArbitrationRoutes(t *testing.T) {
	authStore := reviewAuthStore(t, []string{"review:manage", "arbitration:manage"})
	reviewStore := review.NewMemoryStore()
	reviewStore.AddContext(reviewTenantID, "segment-1", reviewRouteContext())
	router := reviewRouter(authStore, reviewStore)
	managerToken := reviewLogin(t, router, "review_manager")
	firstToken := reviewLogin(t, router, "review_grader")
	secondToken := reviewLogin(t, router, "review_second_grader")
	arbitratorToken := reviewLogin(t, router, "review_arbitrator")

	req := reviewAuthedRequest(http.MethodPut, "/api/v1/exams/exam-1/double-mark-policy", `{"enabled":true,"threshold":1,"resolution_strategy":"average"}`, managerToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"resolution_strategy":"average"`) {
		t.Fatalf("set double mark policy expected 200, got %d %s", rec.Code, rec.Body.String())
	}

	body := `{"answer_segment_id":"segment-1","first_reviewer_id":"` + reviewGraderID + `","second_reviewer_id":"` + reviewSecondGraderID + `"}`
	req = reviewAuthedRequest(http.MethodPost, "/api/v1/double-mark-sessions", body, managerToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated || strings.Count(rec.Body.String(), `"review-task-`) != 2 {
		t.Fatalf("create double mark session expected two review tasks, got %d %s", rec.Code, rec.Body.String())
	}
	session := decodeDoubleMarkSession(t, rec.Body.Bytes())

	req = reviewAuthedRequest(http.MethodPost, "/api/v1/review-tasks/"+session.FirstReviewTaskID+"/submit", `{"score":4,"rubric_selections":[{"point_id":"p1","score":4}]}`, firstToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated || strings.Contains(rec.Body.String(), `"second_score"`) {
		t.Fatalf("first mark should not expose second score, got %d %s", rec.Code, rec.Body.String())
	}

	req = reviewAuthedRequest(http.MethodGet, "/api/v1/review-tasks/"+session.FirstReviewTaskID, "", firstToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `"first_score"`) || strings.Contains(rec.Body.String(), `"second_score"`) {
		t.Fatalf("review task detail must remain blind, got %d %s", rec.Code, rec.Body.String())
	}

	req = reviewAuthedRequest(http.MethodPost, "/api/v1/review-tasks/"+session.SecondReviewTaskID+"/submit", `{"score":1,"rubric_selections":[{"point_id":"p1","score":1}]}`, secondToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"arbitration_task"`) || strings.Contains(rec.Body.String(), `"final_grade"`) {
		t.Fatalf("second mark should create arbitration without final grade, got %d %s", rec.Code, rec.Body.String())
	}
	arbitrationID := decodeArbitrationIDFromSubmit(t, rec.Body.Bytes())

	req = reviewAuthedRequest(http.MethodGet, "/api/v1/arbitration-tasks/"+arbitrationID, "", arbitratorToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"first_score":4`) || !strings.Contains(rec.Body.String(), `"second_score":1`) || !strings.Contains(rec.Body.String(), `"raw_answer":"student wrote the main idea"`) {
		t.Fatalf("arbitration detail expected scores and context, got %d %s", rec.Code, rec.Body.String())
	}

	req = reviewAuthedRequest(http.MethodPost, "/api/v1/arbitration-tasks/"+arbitrationID+"/assign", `{"assigned_to":"`+reviewGraderID+`"}`, arbitratorToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("first reviewer should not be assignable as arbitrator by default, got %d %s", rec.Code, rec.Body.String())
	}

	req = reviewAuthedRequest(http.MethodPost, "/api/v1/arbitration-tasks/"+arbitrationID+"/assign", `{"assigned_to":"`+reviewArbitratorID+`"}`, arbitratorToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"assigned"`) {
		t.Fatalf("assign arbitration expected assigned, got %d %s", rec.Code, rec.Body.String())
	}

	req = reviewAuthedRequest(http.MethodPost, "/api/v1/arbitration-tasks/"+arbitrationID+"/submit", `{"final_score":3,"reason":"rubric evidence supports middle score","student_feedback":"Final score after arbitration."}`, arbitratorToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"source":"arbitration"`) || !strings.Contains(rec.Body.String(), `"score":3`) {
		t.Fatalf("submit arbitration expected final grade, got %d %s", rec.Code, rec.Body.String())
	}

	reviewAssertAuditAction(t, authStore, "review.double_mark_policy_set")
	reviewAssertAuditAction(t, authStore, "review.double_mark_session_created")
	reviewAssertAuditAction(t, authStore, "arbitration.task_created")
	reviewAssertAuditAction(t, authStore, "arbitration.task_assigned")
	reviewAssertAuditAction(t, authStore, "arbitration.submitted")
	reviewAssertAuditAction(t, authStore, "final_grade.created")
}

func reviewRouter(authStore *auth.MemoryStore, reviewStore review.Store) http.Handler {
	cfg := config.Config{
		Service: config.ServiceConfig{Name: "test", Environment: "test", ReadinessTimeout: time.Millisecond},
		Auth:    config.AuthConfig{SessionTTL: time.Hour},
	}
	return NewRouterComplete(cfg, loggerForReviewTest(), nil, authStore, org.NewMemoryStore(), exam.NewMemoryStore(), paper.NewMemoryStore(), files.NewMemoryStore(), files.NewMemoryObjectStorage(), submission.NewMemoryStore(), ocrpkg.NewMemoryStore(), ocrpkg.NewMemoryQueue(), segment.NewMemoryStore(), reviewStore)
}

func createReviewTaskForRoute(t *testing.T, router http.Handler, token string, segmentID string, source string) string {
	t.Helper()
	req := reviewAuthedRequest(http.MethodPost, "/api/v1/review-tasks", `{"answer_segment_id":"`+segmentID+`","source":"`+source+`"}`, token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create review task expected 201, got %d %s", rec.Code, rec.Body.String())
	}
	return decodeReviewTaskID(t, rec.Body.Bytes())
}

func loggerForReviewTest() *logger.Logger {
	return logger.New(io.Discard, "error")
}

func reviewAuthStore(t *testing.T, permissions []string) *auth.MemoryStore {
	return reviewAuthStoreByUser(t, map[string][]string{
		"review_manager":          permissions,
		"review_grader":           permissions,
		"review_second_grader":    permissions,
		"review_arbitrator":       permissions,
		"review_other_arbitrator": permissions,
	})
}

func reviewAuthStoreByUser(t *testing.T, permissionsByUsername map[string][]string) *auth.MemoryStore {
	t.Helper()
	hash, err := auth.HashPassword("ChangeMe123!")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	store := auth.NewMemoryStore()
	for _, user := range []auth.User{
		{
			ID:          reviewManagerID,
			TenantID:    reviewTenantID,
			TenantCode:  "demo",
			Username:    "review_manager",
			DisplayName: "Review Manager",
			Status:      "active",
			Roles:       []string{"teacher"},
			DataScope:   map[string]any{"scope": "school"},
		},
		{
			ID:          reviewGraderID,
			TenantID:    reviewTenantID,
			TenantCode:  "demo",
			Username:    "review_grader",
			DisplayName: "Review Grader",
			Status:      "active",
			Roles:       []string{"grader"},
			DataScope:   map[string]any{"scope": "assigned"},
		},
		{
			ID:          reviewSecondGraderID,
			TenantID:    reviewTenantID,
			TenantCode:  "demo",
			Username:    "review_second_grader",
			DisplayName: "Review Second Grader",
			Status:      "active",
			Roles:       []string{"grader"},
			DataScope:   map[string]any{"scope": "assigned"},
		},
		{
			ID:          reviewArbitratorID,
			TenantID:    reviewTenantID,
			TenantCode:  "demo",
			Username:    "review_arbitrator",
			DisplayName: "Review Arbitrator",
			Status:      "active",
			Roles:       []string{"arbitrator"},
			DataScope:   map[string]any{"scope": "assigned"},
		},
		{
			ID:          reviewOtherArbitratorID,
			TenantID:    reviewTenantID,
			TenantCode:  "demo",
			Username:    "review_other_arbitrator",
			DisplayName: "Review Other Arbitrator",
			Status:      "active",
			Roles:       []string{"arbitrator"},
			DataScope:   map[string]any{"scope": "assigned"},
		},
	} {
		if scopedPermissions, ok := permissionsByUsername[user.Username]; ok {
			user.Permissions = scopedPermissions
		} else {
			user.Permissions = nil
		}
		store.AddUser(auth.UserWithPassword{User: user, PasswordHash: hash})
	}
	return store
}

func reviewLogin(t *testing.T, router http.Handler, username string) string {
	t.Helper()
	raw, _ := json.Marshal(map[string]string{"tenant_code": "demo", "username": username, "password": "ChangeMe123!"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return response.AccessToken
}

func reviewAuthedRequest(method string, path string, body string, token string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func decodeReviewTaskID(t *testing.T, raw []byte) string {
	t.Helper()
	var response struct {
		Task struct {
			ID string `json:"id"`
		} `json:"task"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatalf("decode task: %v", err)
	}
	if response.Task.ID == "" {
		t.Fatalf("missing task id in %s", string(raw))
	}
	return response.Task.ID
}

func assignReviewTaskForRoute(t *testing.T, router http.Handler, token string, taskID string, assignedTo string) {
	t.Helper()
	req := reviewAuthedRequest(http.MethodPost, "/api/v1/review-tasks/"+taskID+"/assign", `{"assigned_to":"`+assignedTo+`"}`, token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("assign review task expected 200, got %d %s", rec.Code, rec.Body.String())
	}
}

func decodeDoubleMarkSession(t *testing.T, raw []byte) struct {
	ID                 string `json:"id"`
	FirstReviewTaskID  string `json:"first_review_task_id"`
	SecondReviewTaskID string `json:"second_review_task_id"`
} {
	t.Helper()
	var response struct {
		DoubleMarkSession struct {
			ID                 string `json:"id"`
			FirstReviewTaskID  string `json:"first_review_task_id"`
			SecondReviewTaskID string `json:"second_review_task_id"`
		} `json:"double_mark_session"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatalf("decode double mark session: %v", err)
	}
	if response.DoubleMarkSession.ID == "" || response.DoubleMarkSession.FirstReviewTaskID == "" || response.DoubleMarkSession.SecondReviewTaskID == "" {
		t.Fatalf("missing double mark session ids in %s", string(raw))
	}
	return response.DoubleMarkSession
}

func decodeArbitrationIDFromSubmit(t *testing.T, raw []byte) string {
	t.Helper()
	var response struct {
		ArbitrationTask struct {
			ID string `json:"id"`
		} `json:"arbitration_task"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatalf("decode arbitration task: %v", err)
	}
	if response.ArbitrationTask.ID == "" {
		t.Fatalf("missing arbitration id in %s", string(raw))
	}
	return response.ArbitrationTask.ID
}

func reviewRouteContext() review.Context {
	return review.Context{
		ExamID:          "exam-1",
		SubmissionID:    "submission-1",
		AnswerSegmentID: "segment-1",
		AnonymousCode:   "ANON-001",
		Question: paper.Question{
			ID:           "question-1",
			TenantID:     reviewTenantID,
			ExamID:       "exam-1",
			QuestionNo:   "Q1",
			QuestionType: "short_answer",
			Score:        5,
		},
		Rubric: paper.Rubric{
			ID:         "rubric-1",
			QuestionID: "question-1",
			Version:    "v1",
			Status:     "approved",
			MaxScore:   5,
			Points: []paper.RubricPoint{
				{ID: "p1", Description: "main idea", Score: 5, Required: true},
			},
		},
		RawAnswer: "student wrote the main idea",
		OCRText:   "student wrote the main idea",
		AISuggestion: map[string]any{
			"suggested_score": 4,
			"mock":            true,
		},
	}
}

func reviewAssertAuditAction(t *testing.T, store *auth.MemoryStore, action string) {
	t.Helper()
	for _, audit := range store.Audits() {
		if audit.Action == action {
			return
		}
	}
	t.Fatalf("missing audit action %s in %#v", action, store.Audits())
}
