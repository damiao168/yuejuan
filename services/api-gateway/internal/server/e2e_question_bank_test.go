package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/commandreceipt"
	"edugrade-enterprise/services/api-gateway/internal/org"
	"edugrade-enterprise/services/api-gateway/internal/questionbank"
	"github.com/google/uuid"
)

// Uses an isolated database, all historical migrations and the production graph.
func TestE2EPostgresQuestionBank(t *testing.T) {
	dsn := os.Getenv("EDUGRADE_E2E_DATABASE_URL")
	if dsn == "" {
		t.Skip("EDUGRADE_E2E_DATABASE_URL is required for real PostgreSQL evidence")
	}
	db := e2eOpenPostgresTestDB(t, dsn)
	e2eApplyPostgresMigrations(t, db)
	e2eActivatePostgresDemoUsers(t, db, []string{"tenant_admin", "teacher", "student"})
	router := e2ePostgresRouter(db)
	token := e2eLoginWithTenant(t, router, "demo", "tenant_admin", "ChangeMe123!")
	teacherToken := e2eLoginWithTenant(t, router, "demo", "teacher", "ChangeMe123!")
	studentToken := e2eLoginWithTenant(t, router, "demo", "student", "ChangeMe123!")
	actor := e2eLookupUserID(t, db, "demo", "tenant_admin")
	var tenant, school string
	if err := db.QueryRow(`SELECT tenant_id::text FROM app_user WHERE id=$1::uuid`, actor).Scan(&tenant); err != nil {
		t.Fatal(err)
	}
	school = e2eString(t, e2ePostJSON(t, router, http.MethodPost, "/api/v1/schools", token, `{"name":"Question bank fixture school","code":"qb-fixture"}`, http.StatusCreated)["school"].(map[string]any), "id")
	grade := e2eString(t, e2ePostJSON(t, router, http.MethodPost, "/api/v1/grades", token, `{"school_id":"`+school+`","name":"QB grade","level_no":8,"academic_year":"2026"}`, http.StatusCreated)["grade"].(map[string]any), "id")
	class := e2eString(t, e2ePostJSON(t, router, http.MethodPost, "/api/v1/classes", token, `{"school_id":"`+school+`","grade_id":"`+grade+`","name":"QB class","code":"qb-class"}`, http.StatusCreated)["class"].(map[string]any), "id")
	if err := org.NewPostgresStore(db).BindTeacherClass(context.Background(), tenant, e2eLookupUserID(t, db, "demo", "teacher"), class); err != nil {
		t.Fatal(err)
	}
	scope := auth.AccessScope{TenantID: tenant, ActorID: actor, TenantWide: true}
	bankInput := questionbank.CreateBankInput{SchoolID: school, Name: "Question bank integration"}
	bankBody := qbJSON(t, bankInput)
	created := qbRequest(t, router, http.MethodPost, "/api/v1/question-banks", token, "bank-create", bankBody, 201)
	bankID := e2eString(t, created["bank"].(map[string]any), "id")
	replay := qbRequest(t, e2ePostgresRouter(db), http.MethodPost, "/api/v1/question-banks", token, "bank-create", bankBody, 201)
	if replay["bank"].(map[string]any)["id"] != bankID {
		t.Fatal("restart lost durable bank receipt")
	}
	qbRequest(t, router, http.MethodPost, "/api/v1/question-banks", token, "", bankBody, 400)
	qbRequest(t, router, http.MethodPost, "/api/v1/question-banks", token, "unknown", strings.TrimSuffix(bankBody, "}")+`,"tenant_id":"`+tenant+`"}`, 400)
	qbRequest(t, router, http.MethodPost, "/api/v1/question-banks", token, "multiple", bankBody+` {}`, 400)
	qbRequest(t, router, http.MethodPost, "/api/v1/question-banks", token, "bank-create", qbJSON(t, questionbank.CreateBankInput{SchoolID: school, Name: "Changed"}), 409)
	qbRequest(t, router, http.MethodGet, "/api/v1/question-banks", studentToken, "", "", 403)
	qbRequest(t, router, http.MethodGet, "/api/v1/question-banks", "", "", "", 401)
	content := questionbank.Content{QuestionType: "single_choice", AssessmentArchetype: "selected_response", Stem: "Synthetic private stem", Options: []string{"First", "Second"}, DefaultScore: 2.5, KnowledgePoints: []string{"fixture.math"}, Metadata: questionbank.Metadata{SubjectCode: "mathematics", EducationStage: "junior", GradeScope: "grade_8"}}
	itemPath := "/api/v1/question-banks/" + bankID + "/items"
	result := qbRequest(t, router, http.MethodPost, itemPath, token, "item-create", qbJSON(t, questionbank.CreateItemInput{ItemCode: "M1", Content: content}), 201)
	itemID := e2eString(t, result["item"].(map[string]any), "id")
	versionID := e2eString(t, result["version"].(map[string]any), "id")
	if result["item"].(map[string]any)["current_published_version_id"] != nil {
		t.Fatal("draft became published")
	}
	qbRequest(t, router, http.MethodPost, itemPath, token, "duplicate-item", qbJSON(t, questionbank.CreateItemInput{ItemCode: "M1", Content: content}), 409)
	invalid := content
	invalid.QuestionType = "unknown"
	qbRequest(t, router, http.MethodPost, itemPath, token, "bad-enum", qbJSON(t, questionbank.CreateItemInput{ItemCode: "BAD", Content: invalid}), 400)
	invalid = content
	invalid.DefaultScore = 0.001
	qbRequest(t, router, http.MethodPost, itemPath, token, "bad-score", qbJSON(t, questionbank.CreateItemInput{ItemCode: "BAD", Content: invalid}), 400)
	page := qbRequest(t, router, http.MethodGet, "/api/v1/question-banks", teacherToken, "", "", 200)
	if page["total"] != float64(0) || len(page["banks"].([]any)) != 0 {
		t.Fatal("same tenant unbound bank count leaked")
	}
	for _, path := range []string{"/api/v1/question-banks/" + bankID, itemPath, "/api/v1/question-bank/items/" + itemID, "/api/v1/question-bank/items/" + itemID + "/versions", "/api/v1/question-bank/versions/" + versionID} {
		qbRequest(t, router, http.MethodGet, path, teacherToken, "", "", 404)
	}
	qbRequest(t, router, http.MethodPatch, "/api/v1/question-bank/versions/"+versionID, teacherToken, "denied-edit", qbJSON(t, questionbank.UpdateVersionInput{ExpectedRevision: 1, Content: content}), 404)
	teacherBank := qbRequest(t, router, http.MethodPost, "/api/v1/question-banks", teacherToken, "teacher-bank", qbJSON(t, questionbank.CreateBankInput{SchoolID: school, Name: "Teacher private bank"}), 201)["bank"].(map[string]any)
	teacherBankID := e2eString(t, teacherBank, "id")
	qbRequest(t, router, http.MethodPost, "/api/v1/question-banks/"+teacherBankID+"/items", teacherToken, "teacher-item", qbJSON(t, questionbank.CreateItemInput{ItemCode: "T1", Content: content}), 201)
	qbRequest(t, router, http.MethodGet, "/api/v1/question-banks/"+teacherBankID, token, "", "", 404)
	if privatePage := qbRequest(t, router, http.MethodGet, "/api/v1/question-banks", token, "", "", 200); privatePage["total"] != float64(1) {
		t.Fatal("tenant administrator inherited private teacher bank")
	}

	store := questionbank.NewPostgresStore(db)
	ctx := context.Background()
	t.Run("concurrent version allocation and durable replay", func(t *testing.T) {
		type outcome struct {
			v   questionbank.Version
			err error
		}
		out := make(chan outcome, 8)
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				v, err := store.CreateVersion(commandreceipt.WithID(ctx, "clone-"+string(rune('a'+i))), scope, itemID, questionbank.CreateVersionInput{SourceVersionID: versionID})
				out <- outcome{v, err}
			}(i)
		}
		wg.Wait()
		close(out)
		numbers := map[int]bool{}
		for r := range out {
			if r.err != nil {
				t.Fatal(r.err)
			}
			if numbers[r.v.VersionNo] || r.v.VersionNo < 2 || r.v.VersionNo > 9 {
				t.Fatalf("nonunique allocation: %+v", r.v)
			}
			numbers[r.v.VersionNo] = true
		}
		sameCtx := commandreceipt.WithID(ctx, "clone-same")
		out = make(chan outcome, 4)
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				v, err := store.CreateVersion(sameCtx, scope, itemID, questionbank.CreateVersionInput{SourceVersionID: versionID})
				out <- outcome{v, err}
			}()
		}
		wg.Wait()
		close(out)
		var sameID string
		for r := range out {
			if r.err != nil {
				t.Fatal(r.err)
			}
			if sameID == "" {
				sameID = r.v.ID
			}
			if r.v.ID != sameID || r.v.VersionNo != 10 {
				t.Fatal("same command allocated twice")
			}
		}
		versions, err := store.ListVersions(ctx, scope, itemID, questionbank.Filter{Limit: 3, Offset: 3})
		if err != nil || versions.Total != 10 || len(versions.Versions) != 3 || versions.Versions[0].VersionNo != 7 {
			t.Fatalf("history page: %+v %v", versions, err)
		}
	})
	t.Run("competing optimistic edits", func(t *testing.T) {
		failures := make(chan error, 2)
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				next := content
				next.Stem = "Concurrent " + string(rune('a'+i))
				_, err := store.UpdateVersion(commandreceipt.WithID(ctx, "edit-"+string(rune('a'+i))), scope, versionID, questionbank.UpdateVersionInput{ExpectedRevision: 1, Content: next})
				failures <- err
			}(i)
		}
		wg.Wait()
		close(failures)
		successes, conflicts := 0, 0
		for err := range failures {
			if err == nil {
				successes++
			} else if errors.Is(err, questionbank.ErrConflict) {
				conflicts++
			} else {
				t.Fatal(err)
			}
		}
		if successes != 1 || conflicts != 1 {
			t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
		}
	})
	t.Run("composite foreign keys and draft guard", func(t *testing.T) {
		otherTenant := "00000000-0000-0000-0000-000000000001"
		otherActor := e2eLookupUserID(t, db, "platform", "platform_admin")
		mustReject := func(query string, args ...any) {
			t.Helper()
			if _, err := db.Exec(query, args...); err == nil {
				t.Fatal("database accepted invalid identity/state")
			}
		}
		mustReject(`INSERT INTO question_bank_item(tenant_id,bank_id,item_code,subject_code,grade_scope,created_by) VALUES($1::uuid,$2::uuid,'CROSS','mathematics','grade_8',$3::uuid)`, otherTenant, bankID, otherActor)
		mustReject(`INSERT INTO question_bank(tenant_id,school_id,name,created_by) VALUES($1::uuid,$2::uuid,'CROSS SCHOOL',$3::uuid)`, otherTenant, school, otherActor)
		second, err := store.CreateItem(commandreceipt.WithID(ctx, "second-item"), scope, bankID, questionbank.CreateItemInput{ItemCode: "M2", Content: content})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = store.CreateVersion(commandreceipt.WithID(ctx, "wrong-source"), scope, itemID, questionbank.CreateVersionInput{SourceVersionID: second.Version.ID}); !errors.Is(err, questionbank.ErrInvalidInput) {
			t.Fatalf("wrong source: %v", err)
		}
		mustReject(`UPDATE question_bank_item_version SET source_version_id=$2::uuid,revision=revision+1 WHERE id=$1::uuid`, versionID, second.Version.ID)
		mustReject(`INSERT INTO question_bank_item_version(tenant_id,item_id,version_no,source_version_id,author_id,content,content_hash) SELECT tenant_id,$2::uuid,999,id,author_id,content,content_hash FROM question_bank_item_version WHERE id=$1::uuid`, second.Version.ID, itemID)
		mustReject(`UPDATE question_bank_item_version SET content=jsonb_set(content,'{stem}','null'::jsonb),revision=revision+1 WHERE id=$1::uuid`, versionID)
		mustReject(`UPDATE question_bank_item_version SET content=jsonb_set(content,'{metadata,copyright}','"unknown-value"'::jsonb),revision=revision+1 WHERE id=$1::uuid`, versionID)
		mustReject(`UPDATE question_bank_item SET current_published_version_id=$2::uuid WHERE id=$1::uuid`, itemID, versionID)
		mustReject(`UPDATE question_bank_item_version SET workflow_status='published',revision=revision+1 WHERE id=$1::uuid`, versionID)
		mustReject(`DELETE FROM question_bank_item_version WHERE id=$1::uuid`, versionID)
	})
	t.Run("archive locks content mutations", func(t *testing.T) {
		b, err := store.UpdateBank(commandreceipt.WithID(ctx, "archive"), scope, bankID, questionbank.UpdateBankInput{ExpectedRevision: 1, Name: bankInput.Name, Status: "archived"})
		if err != nil {
			t.Fatal(err)
		}
		_, err = store.CreateVersion(commandreceipt.WithID(ctx, "archived-clone"), scope, itemID, questionbank.CreateVersionInput{SourceVersionID: versionID})
		if !errors.Is(err, questionbank.ErrLocked) {
			t.Fatalf("archived clone: %v", err)
		}
		_, err = store.CreateItem(commandreceipt.WithID(ctx, "archived-item"), scope, bankID, questionbank.CreateItemInput{ItemCode: "ARCHIVED", Content: content})
		if !errors.Is(err, questionbank.ErrLocked) {
			t.Fatalf("archived item: %v", err)
		}
		_, err = store.UpdateBank(commandreceipt.WithID(ctx, "reactivate"), scope, bankID, questionbank.UpdateBankInput{ExpectedRevision: b.Revision, Name: b.Name, Status: "active"})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("transaction rollback includes audit outbox and receipt", func(t *testing.T) {
		before := qbCounts(t, db)
		_, err := db.Exec(`CREATE FUNCTION qb_fail_outbox() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.event_type='question_bank.item.create' THEN RAISE EXCEPTION 'synthetic outbox fault'; END IF; RETURN NEW; END $$; CREATE TRIGGER qb_fail BEFORE INSERT ON event_outbox FOR EACH ROW EXECUTE FUNCTION qb_fail_outbox()`)
		if err != nil {
			t.Fatal(err)
		}
		faultCtx := commandreceipt.WithID(ctx, "fault-item")
		in := questionbank.CreateItemInput{ItemCode: "FAULT", Content: content}
		_, err = store.CreateItem(faultCtx, scope, bankID, in)
		if err == nil {
			t.Fatal("fault was not injected")
		}
		if after := qbCounts(t, db); after != before {
			t.Fatalf("partial transaction persisted: before=%v after=%v", before, after)
		}
		if _, err = db.Exec(`DROP TRIGGER qb_fail ON event_outbox; DROP FUNCTION qb_fail_outbox()`); err != nil {
			t.Fatal(err)
		}
		if _, err = store.CreateItem(faultCtx, scope, bankID, in); err != nil {
			t.Fatalf("same failed command must be retryable: %v", err)
		}
	})
	t.Run("runtime RLS fails closed and follows tenant", func(t *testing.T) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if _, err = tx.Exec(`SET LOCAL ROLE edugrade_tenant_runtime`); err != nil {
			t.Fatal(err)
		}
		count := func(table string) int {
			t.Helper()
			var n int
			if err := tx.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
				t.Fatal(err)
			}
			return n
		}
		for _, table := range []string{"question_bank", "question_bank_acl", "question_bank_item", "question_bank_item_version"} {
			if count(table) != 0 {
				t.Fatal("RLS missing tenant exposed " + table)
			}
		}
		if _, err = tx.Exec(`SELECT set_config('edugrade.tenant_id',$1,true)`, tenant); err != nil {
			t.Fatal(err)
		}
		if count("question_bank_item_version") < 10 {
			t.Fatal("runtime role cannot read scoped drafts")
		}
		if _, err = tx.Exec(`SELECT set_config('edugrade.tenant_id','00000000-0000-0000-0000-000000000001',true)`); err != nil {
			t.Fatal(err)
		}
		for _, table := range []string{"question_bank", "question_bank_acl", "question_bank_item", "question_bank_item_version"} {
			if count(table) != 0 {
				t.Fatal("RLS cross tenant exposed " + table)
			}
		}
	})
	t.Run("new tenant provisioning inherits action permissions", func(t *testing.T) {
		provisioned, err := org.NewPostgresStore(db).CreateTenant(ctx, org.TenantProvision{Name: "QB provision fixture", Code: "qb-" + strings.ReplaceAll(uuid.NewString(), "-", ""), AdminUsername: "admin", AdminDisplayName: "Fixture admin", PasswordHash: "fixture-only"})
		if err != nil {
			t.Fatal(err)
		}
		var count int
		err = db.QueryRow(`SELECT count(*) FROM role_permission rp JOIN role r ON r.tenant_id=rp.tenant_id AND r.id=rp.role_id JOIN permission p ON p.tenant_id=rp.tenant_id AND p.id=rp.permission_id WHERE r.tenant_id=$1::uuid AND r.code='tenant_admin' AND p.code LIKE 'question_bank:%' AND rp.deleted_at IS NULL`, provisioned.ID).Scan(&count)
		if err != nil || count != 8 {
			t.Fatalf("future tenant permissions: %d %v", count, err)
		}
	})
	t.Run("read revocation also denies HTTP receipt replay", func(t *testing.T) {
		outsideSchool := scope
		outsideSchool.TenantWide = false
		outsideSchool.SchoolIDs = []string{uuid.NewString()}
		if _, err := store.GetVersion(ctx, outsideSchool, versionID); !errors.Is(err, questionbank.ErrNotFound) {
			t.Fatalf("creator ACL bypassed school boundary: %v", err)
		}
		if _, err := store.CreateVersion(commandreceipt.WithID(ctx, "clone-same"), outsideSchool, itemID, questionbank.CreateVersionInput{SourceVersionID: versionID}); !errors.Is(err, questionbank.ErrNotFound) {
			t.Fatalf("replay bypassed school boundary: %v", err)
		}
		if _, err := store.CreateBank(commandreceipt.WithID(ctx, "outside-school"), outsideSchool, bankInput); !errors.Is(err, auth.ErrForbidden) {
			t.Fatalf("bank creation bypassed school boundary: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM question_bank_acl WHERE bank_id=$1::uuid AND user_id=$2::uuid AND action='read'`, bankID, actor); err != nil {
			t.Fatal(err)
		}
		qbRequest(t, router, http.MethodPost, itemPath, token, "item-create", qbJSON(t, questionbank.CreateItemInput{ItemCode: "M1", Content: content}), 404)
		qbRequest(t, e2ePostgresRouter(db), http.MethodPost, "/api/v1/question-banks", token, "bank-create", bankBody, 404)
		if _, err := store.CreateVersion(commandreceipt.WithID(ctx, "clone-same"), scope, itemID, questionbank.CreateVersionInput{SourceVersionID: versionID}); !errors.Is(err, questionbank.ErrNotFound) {
			t.Fatalf("replay bypassed revoked ACL: %v", err)
		}
	})
}

func qbJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
func qbRequest(t *testing.T, router http.Handler, method, path, token, key, body string, want int) map[string]any {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("%s %s want=%d got=%d body=%s", method, path, want, rec.Code, rec.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func qbCounts(t *testing.T, db *sql.DB) [5]int {
	t.Helper()
	var result [5]int
	for i, table := range []string{"question_bank_item", "question_bank_item_version", "audit_log", "event_outbox", "business_command_receipt"} {
		if err := db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&result[i]); err != nil {
			t.Fatal(err)
		}
	}
	return result
}
