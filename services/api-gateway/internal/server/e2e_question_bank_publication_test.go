package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"sync"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/commandreceipt"
	"edugrade-enterprise/services/api-gateway/internal/org"
	"edugrade-enterprise/services/api-gateway/internal/paper"
	"edugrade-enterprise/services/api-gateway/internal/questionbank"
)

func TestE2EPostgresQuestionBankPublication(t *testing.T) {
	dsn := os.Getenv("EDUGRADE_E2E_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires isolated PostgreSQL")
	}
	db := e2eOpenPostgresTestDB(t, dsn)
	e2eApplyPostgresMigrations(t, db)
	e2eActivatePostgresDemoUsers(t, db, []string{"tenant_admin", "teacher"})
	router := e2ePostgresRouter(db)
	token := e2eLoginWithTenant(t, router, "demo", "tenant_admin", "ChangeMe123!")
	reviewerToken := e2eLoginWithTenant(t, router, "demo", "teacher", "ChangeMe123!")
	actor := e2eLookupUserID(t, db, "demo", "tenant_admin")
	reviewer := e2eLookupUserID(t, db, "demo", "teacher")
	var tenant string
	if err := db.QueryRow(`SELECT tenant_id::text FROM app_user WHERE id=$1`, actor).Scan(&tenant); err != nil {
		t.Fatal(err)
	}
	school := e2eString(t, e2ePostJSON(t, router, http.MethodPost, "/api/v1/schools", token, `{"name":"Publication school","code":"qb-pub"}`, 201)["school"].(map[string]any), "id")
	grade := e2eString(t, e2ePostJSON(t, router, http.MethodPost, "/api/v1/grades", token, `{"school_id":"`+school+`","name":"Pub grade","level_no":8,"academic_year":"2026"}`, 201)["grade"].(map[string]any), "id")
	class := e2eString(t, e2ePostJSON(t, router, http.MethodPost, "/api/v1/classes", token, `{"school_id":"`+school+`","grade_id":"`+grade+`","name":"Pub class","code":"pub-class"}`, 201)["class"].(map[string]any), "id")
	if err := org.NewPostgresStore(db).BindTeacherClass(context.Background(), tenant, reviewer, class); err != nil {
		t.Fatal(err)
	}
	store := questionbank.NewPostgresStore(db)
	scope := auth.AccessScope{TenantID: tenant, ActorID: actor, TenantWide: true}
	ctx := context.Background()
	bank, err := store.CreateBank(commandreceipt.WithID(ctx, "pub-bank"), scope, questionbank.CreateBankInput{SchoolID: school, Name: "Publication bank"})
	if err != nil {
		t.Fatal(err)
	}
	for n, user := range []string{actor, reviewer} {
		bank, err = store.BindReviewers(commandreceipt.WithID(ctx, "pub-bind-"+user), scope, bank.ID, questionbank.ReviewerBinding{ExpectedRevision: bank.Revision, UserID: user, Read: true, Review: true, Publish: n == 0})
		if err != nil {
			t.Fatal(err)
		}
	}
	content := questionbank.Content{QuestionType: "short_answer", AssessmentArchetype: "short_constructed", Stem: "Synthetic publication <question>", DefaultScore: 5, Options: []string{}, KnowledgePoints: []string{"test"}, Metadata: questionbank.Metadata{SubjectCode: "mathematics", EducationStage: "junior", GradeScope: "grade_8", Copyright: "owned"}}
	rubric := &paper.RubricInput{MaxScore: 5, Points: []paper.RubricPoint{{ID: "P1", Description: "Explain the concept", Score: 5, Required: true, EvidenceRequirements: []paper.EvidenceRequirement{{Type: "concept", Target: "synthetic"}}}}, Deductions: []any{"one deduction"}, Examples: []any{"one example"}}
	scoring := questionbank.Scoring{Answer: &paper.AnswerKeyInput{StandardAnswer: "synthetic", EquivalentAnswers: []any{"equivalent"}, Tolerance: map[string]any{}}, Solution: &paper.SolutionInput{RawText: "Synthetic explanation", Steps: []paper.SolutionStep{{StepNo: 1, Content: "First step"}}}, Rubric: rubric, Assets: []questionbank.Asset{}, UsePolicy: "exam_allowed"}
	create := func(code, kind string) questionbank.ItemResult {
		out, err := store.CreateItem(commandreceipt.WithID(ctx, "pub-create-"+code), scope, bank.ID, questionbank.CreateItemInput{ItemCode: code, Kind: kind, Content: content})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	save := func(v questionbank.Version, s questionbank.Scoring, key string) questionbank.Version {
		out, err := store.UpdateScoring(commandreceipt.WithID(ctx, key), scope, v.ID, questionbank.UpdateScoringInput{ExpectedRevision: v.Revision, Scoring: s})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	decode := func(value map[string]any) questionbank.Version {
		raw, _ := json.Marshal(value["version"])
		var v questionbank.Version
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	decision := func(v questionbank.Version, action, authToken, key string, status int) questionbank.Version {
		out := qbRequest(t, router, http.MethodPost, "/api/v1/question-bank/versions/"+v.ID+"/"+action, authToken, key, qbJSON(t, questionbank.ReviewInput{ExpectedRevision: v.Revision, BundleHash: v.BundleHash, Comment: "Synthetic decision"}), status)
		if status != 200 {
			return v
		}
		return decode(out)
	}
	publish := func(v questionbank.Version, prefix string) questionbank.Version {
		v = decision(v, "submit-review", token, prefix+"-submit", 200)
		v = decision(v, "approve", reviewerToken, prefix+"-approve", 200)
		return decision(v, "publish", token, prefix+"-publish", 200)
	}
	template := create("T1", "rubric_template")
	template.Version = save(template.Version, scoring, "template-score")
	template.Version = publish(template.Version, "template")
	item := create("Q1", "question")
	sourceTemplate := template.Version.ID
	scoring.TemplateVersionID = &sourceTemplate
	var assetID string
	err = db.QueryRow(`INSERT INTO file_asset(tenant_id,school_id,owner_type,owner_id,original_name,content_type,size_bytes,hash_sha256,storage_bucket,storage_key,visibility,uploaded_by) VALUES($1,$2,'paper',$2,'fixture.txt','text/plain',7,repeat('a',64),'fixture','fixture/pub.txt','private',$3) RETURNING id::text`, tenant, school, actor).Scan(&assetID)
	if err != nil {
		t.Fatal(err)
	}
	scoring.Assets = []questionbank.Asset{{FileAssetID: assetID, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Name: "fixture.txt", ContentType: "text/plain"}}
	v := save(item.Version, scoring, "question-score")
	v = decision(v, "submit-review", token, "question-submit", 200)
	t.Run("independent approval and return invalidation", func(t *testing.T) {
		decision(v, "approve", token, "self-approve", 400)
		if _, err := store.UpdateScoring(commandreceipt.WithID(ctx, "locked-score"), scope, v.ID, questionbank.UpdateScoringInput{ExpectedRevision: v.Revision, Scoring: scoring}); err != questionbank.ErrLocked {
			t.Fatalf("reviewing edit: %v", err)
		}
		old := v
		reviewerScope := auth.AccessScope{TenantID: tenant, ActorID: reviewer, SchoolIDs: []string{school}, ClassIDs: []string{class}}
		type transitionResult struct {
			version questionbank.Version
			err     error
		}
		results := make(chan transitionResult, 2)
		var wg sync.WaitGroup
		for _, key := range []string{"approve-race-a", "approve-race-b"} {
			wg.Add(1)
			go func(key string) {
				defer wg.Done()
				out, err := store.Transition(commandreceipt.WithID(ctx, key), reviewerScope, v.ID, "approve", questionbank.ReviewInput{ExpectedRevision: v.Revision, BundleHash: v.BundleHash, Comment: "Concurrent approval"})
				results <- transitionResult{version: out, err: err}
			}(key)
		}
		wg.Wait()
		close(results)
		succeeded, rejected := 0, 0
		for result := range results {
			switch {
			case result.err == nil:
				succeeded++
				v = result.version
			case errors.Is(result.err, questionbank.ErrLocked):
				rejected++
			default:
				t.Fatalf("concurrent approval: %v", result.err)
			}
		}
		if succeeded != 1 || rejected != 1 {
			t.Fatalf("concurrent approval results: success=%d rejected=%d", succeeded, rejected)
		}
		v = decision(v, "return-to-draft", token, "withdraw", 200)
		v = save(v, scoring, "score-after-return")
		decision(v, "publish", token, "no-approval", 409)
		decision(old, "approve", reviewerToken, "stale-approval", 409)
		v = decision(v, "submit-review", token, "question-final-submit", 200)
		v = decision(v, "approve", reviewerToken, "question-final-approve", 200)
		results = make(chan transitionResult, 2)
		for _, key := range []string{"publish-race-a", "publish-race-b"} {
			wg.Add(1)
			go func(key string) {
				defer wg.Done()
				out, err := store.Transition(commandreceipt.WithID(ctx, key), scope, v.ID, "publish", questionbank.ReviewInput{ExpectedRevision: v.Revision, BundleHash: v.BundleHash, Comment: "Concurrent publish"})
				results <- transitionResult{version: out, err: err}
			}(key)
		}
		wg.Wait()
		close(results)
		succeeded, rejected = 0, 0
		for result := range results {
			switch {
			case result.err == nil:
				succeeded++
				v = result.version
			case errors.Is(result.err, questionbank.ErrLocked):
				rejected++
			default:
				t.Fatalf("concurrent publish: %v", result.err)
			}
		}
		if succeeded != 1 || rejected != 1 {
			t.Fatalf("concurrent publish results: success=%d rejected=%d", succeeded, rejected)
		}
	})
	t.Run("published child facts and bindings are immutable", func(t *testing.T) {
		for _, sql := range []string{`UPDATE question_bank_item_version SET content=jsonb_set(content,'{stem}','"mutated"'),revision=revision+1 WHERE id=$1`, `UPDATE question_bank_answer_version SET facts='{}' WHERE version_id=$1`, `DELETE FROM question_bank_rubric_version WHERE version_id=$1`, `UPDATE question_bank_item_asset SET sha256=repeat('b',64) WHERE version_id=$1`, `UPDATE question_bank_review SET comment='mutated' WHERE version_id=$1`} {
			if _, err := db.Exec(sql, v.ID); err == nil {
				t.Fatal("direct mutation allowed: " + sql)
			}
		}
		if _, err := db.Exec(`UPDATE file_asset SET lifecycle_status='pending_delete',revision=revision+1 WHERE id=$1`, assetID); err == nil {
			t.Fatal("referenced asset deletion allowed")
		}
		detail, err := store.GetVersion(ctx, scope, v.ID)
		if err != nil || detail.AnswerVersionID == nil || detail.RubricVersionID == nil {
			t.Fatalf("explicit children: %+v %v", detail, err)
		}
		// Same canonical facts use the same hash in development and PostgreSQL.
		memory := questionbank.NewMemoryStore()
		mb, _ := memory.CreateBank(commandreceipt.WithID(ctx, "m-bank"), scope, questionbank.CreateBankInput{SchoolID: school, Name: "Memory"})
		mi, _ := memory.CreateItem(commandreceipt.WithID(ctx, "m-item"), scope, mb.ID, questionbank.CreateItemInput{ItemCode: "M1", Content: content})
		ms := scoring
		ms.TemplateVersionID = nil
		ms.Assets = nil
		mv, err := memory.UpdateScoring(commandreceipt.WithID(ctx, "m-score"), scope, mi.Version.ID, questionbank.UpdateScoringInput{ExpectedRevision: 1, Scoring: ms})
		if err != nil {
			t.Fatal(err)
		}
		var expected string
		raw, _ := json.Marshal(mv.Scoring)
		craw, _ := json.Marshal(mv.Content)
		if err = db.QueryRow(`SELECT question_bank_bundle_hash($1,$2)`, craw, raw).Scan(&expected); err != nil {
			t.Fatal(err)
		}
		if expected != mv.BundleHash {
			t.Fatalf("canonical drift: %s != %s", expected, mv.BundleHash)
		}
	})
	exam := e2ePostJSON(t, router, http.MethodPost, "/api/v1/exams", token, `{"school_id":"`+school+`","name":"Bank copy exam","subject":"数学","exam_type":"unit","total_score":5,"grading_mode":"human_review_required","publish_policy":"after_review","class_ids":["`+class+`"]}`, 201)["exam"].(map[string]any)
	examID := e2eString(t, exam, "id")
	in := questionbank.MaterializeInput{ExpectedRevision: 1, Selections: []questionbank.MaterializeSelection{{VersionID: v.ID, QuestionNo: "1", SortOrder: 1}}}
	t.Run("atomic failure recovery and durable receipt", func(t *testing.T) {
		outsideScope := auth.AccessScope{TenantID: tenant, ActorID: actor, SchoolIDs: []string{"00000000-0000-0000-0000-000000000001"}}
		if _, err := store.Materialize(commandreceipt.WithID(ctx, "copy-outside-scope"), outsideScope, examID, in); !errors.Is(err, questionbank.ErrNotFound) {
			t.Fatalf("cross-scope copy: %v", err)
		}
		if _, err := db.Exec(`CREATE FUNCTION qb_fail_solution() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic failure'; END $$; CREATE TRIGGER qb_fail_solution BEFORE INSERT ON question_solution FOR EACH ROW EXECUTE FUNCTION qb_fail_solution()`); err != nil {
			t.Fatal(err)
		}
		qbRequest(t, router, http.MethodPost, "/api/v1/exams/"+examID+"/questions/materialize-from-bank", token, "copy-recover", qbJSON(t, in), 503)
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM question WHERE exam_id=$1`, examID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("partial copy: %d %v", count, err)
		}
		if _, err := db.Exec(`DROP TRIGGER qb_fail_solution ON question_solution; DROP FUNCTION qb_fail_solution()`); err != nil {
			t.Fatal(err)
		}
		out := qbRequest(t, router, http.MethodPost, "/api/v1/exams/"+examID+"/questions/materialize-from-bank", token, "copy-recover", qbJSON(t, in), 201)
		replay := qbRequest(t, e2ePostgresRouter(db), http.MethodPost, "/api/v1/exams/"+examID+"/questions/materialize-from-bank", token, "copy-recover", qbJSON(t, in), 201)
		if out["questions"].([]any)[0].(map[string]any)["id"] != replay["questions"].([]any)[0].(map[string]any)["id"] {
			t.Fatal("copy replay changed IDs")
		}
		qbRequest(t, router, http.MethodPost, "/api/v1/exams/"+examID+"/questions/materialize-from-bank", token, "copy-stale", qbJSON(t, in), 409)
	})
	t.Run("template versions and bank retirement cannot change exam facts", func(t *testing.T) {
		questions, err := paper.NewPostgresStore(db).ListQuestions(ctx, tenant, examID)
		if err != nil || len(questions) != 1 {
			t.Fatalf("copied questions: %+v %v", questions, err)
		}
		before := qbJSON(t, questions)
		if questions[0].AnswerArea != nil || questions[0].AnswerKey == nil || questions[0].Solution == nil || questions[0].Rubric == nil || questions[0].SourceContentHash != v.BundleHash {
			t.Fatal("incomplete frozen copy")
		}
		// Snapshot freeze reads only copied assessment/rubric/source facts. Missing
		// answer area still fails the public paper readiness check.
		validation, err := paper.NewPostgresStore(db).ValidateConfig(ctx, tenant, examID)
		if err != nil || validation.Valid {
			t.Fatalf("missing area bypassed validation: %+v %v", validation, err)
		}
		var snapshotID, hash string
		if err := db.QueryRow(`SELECT assessment_freeze_question_snapshot($1,$2,$3)::text`, tenant, examID, questions[0].ID).Scan(&snapshotID); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`SELECT content_hash FROM exam_question_snapshot WHERE id=$1 AND snapshot_version=2 AND source_snapshot_json->>'bundle_hash'=$2`, snapshotID, v.BundleHash).Scan(&hash); err != nil {
			t.Fatal(err)
		}
		next, err := store.CreateVersion(commandreceipt.WithID(ctx, "template-v2"), scope, template.Item.ID, questionbank.CreateVersionInput{SourceVersionID: template.Version.ID})
		if err != nil {
			t.Fatal(err)
		}
		changed := scoring
		changed.TemplateVersionID = nil
		changed.Assets = nil
		changed.Rubric = &paper.RubricInput{MaxScore: 5, Points: []paper.RubricPoint{{ID: "NEW", Description: "New template fact", Score: 5}}}
		next = save(next, changed, "template-v2-score")
		publish(next, "template-v2")
		next, err = store.CreateVersion(commandreceipt.WithID(ctx, "question-v2"), scope, item.Item.ID, questionbank.CreateVersionInput{SourceVersionID: v.ID})
		if err != nil {
			t.Fatal(err)
		}
		next = save(next, changed, "question-v2-score")
		publish(next, "question-v2")
		if _, err := db.Exec(`UPDATE question_bank_item SET status='retired' WHERE id=$1`, item.Item.ID); err != nil {
			t.Fatal(err)
		}
		after, err := paper.NewPostgresStore(db).ListQuestions(ctx, tenant, examID)
		if err != nil || before != qbJSON(t, after) {
			t.Fatalf("bank mutation changed exam: %v", err)
		}
		var afterHash string
		if err := db.QueryRow(`SELECT content_hash FROM exam_question_snapshot WHERE id=$1`, snapshotID).Scan(&afterHash); err != nil || hash != afterHash {
			t.Fatal("bank changed frozen target hash")
		}
		if _, err := db.Exec(`UPDATE question SET source_content_hash=repeat('b',64) WHERE id=$1`, questions[0].ID); err == nil {
			t.Fatal("source provenance mutated")
		}
		if _, err := db.Exec(`UPDATE exam SET status='ready' WHERE tenant_id=$1 AND id=$2`, tenant, examID); err != nil {
			t.Fatal(err)
		}
		locked := questionbank.MaterializeInput{ExpectedRevision: 2, Selections: []questionbank.MaterializeSelection{{VersionID: v.ID, QuestionNo: "2", SortOrder: 2}}}
		if _, err := store.Materialize(commandreceipt.WithID(ctx, "copy-ready-exam"), scope, examID, locked); !errors.Is(err, questionbank.ErrLocked) {
			t.Fatalf("ready exam copy: %v", err)
		}
	})
}
