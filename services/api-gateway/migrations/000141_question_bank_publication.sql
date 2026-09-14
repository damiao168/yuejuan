-- Publication is a content-bound decision. Append-only child facts retain each
-- scoring revision, including drafts, and never point at mutable templates.
ALTER TABLE question_bank_acl DROP CONSTRAINT question_bank_acl_action_check;
ALTER TABLE question_bank_acl ADD CHECK(action IN ('read','create','edit','manage','review','publish'));
ALTER TABLE question_bank_item ADD COLUMN kind TEXT NOT NULL DEFAULT 'question' CHECK(kind IN ('question','rubric_template'));
ALTER TABLE question_bank_item DROP CONSTRAINT question_bank_item_current_published_version_id_check;
ALTER TABLE question_bank_item_version DROP CONSTRAINT question_bank_item_version_workflow_status_check;
ALTER TABLE question_bank_item_version ADD CHECK(workflow_status IN ('draft','reviewing','approved','published'));
ALTER TABLE question_bank_item_version ADD COLUMN scoring JSONB NOT NULL DEFAULT '{"answer":null,"solution":null,"rubric":null,"template_version_id":null,"assets":[],"use_policy":"practice_only"}';
ALTER TABLE question_bank_item_version ADD COLUMN bundle_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE question_bank_item_version ADD COLUMN last_review_id UUID;
ALTER TABLE question_bank_item_version ADD CHECK((jsonb_typeof(scoring)='object' AND scoring ?& ARRAY['answer','solution','rubric','template_version_id','assets','use_policy'] AND jsonb_typeof(scoring->'answer') IN ('object','null') AND jsonb_typeof(scoring->'solution') IN ('object','null') AND jsonb_typeof(scoring->'rubric') IN ('object','null') AND jsonb_typeof(scoring->'template_version_id') IN ('string','null') AND jsonb_typeof(scoring->'assets')='array' AND jsonb_array_length(scoring->'assets')<=32 AND scoring->>'use_policy' IN ('practice_only','exam_allowed')) IS TRUE);

CREATE FUNCTION question_bank_canonical_json(value JSONB) RETURNS TEXT LANGUAGE SQL IMMUTABLE STRICT AS $$
 SELECT CASE jsonb_typeof(value)
 WHEN 'object' THEN '{'||COALESCE((SELECT string_agg(to_jsonb(key)::text||':'||question_bank_canonical_json(v),',' ORDER BY key COLLATE "C") FROM jsonb_each(value) x(key,v)),'')||'}'
 WHEN 'array' THEN '['||COALESCE((SELECT string_agg(question_bank_canonical_json(v),',' ORDER BY n) FROM jsonb_array_elements(value) WITH ORDINALITY x(v,n)),'')||']'
 ELSE value::text END
$$;
CREATE FUNCTION question_bank_bundle_hash(content JSONB, scoring JSONB) RETURNS TEXT LANGUAGE SQL IMMUTABLE STRICT AS $$
 SELECT encode(digest(question_bank_canonical_json(jsonb_build_object('schema_version',2,'content',content,'scoring',jsonb_set(scoring-'template_version_id','{assets}',COALESCE((SELECT jsonb_agg(value-'file_asset_id' ORDER BY n) FROM jsonb_array_elements(scoring->'assets') WITH ORDINALITY x(value,n)),'[]')))),'sha256'),'hex')
$$;
-- Upgrade old draft fingerprints without rewriting their schema-1 content hash.
DROP TRIGGER question_bank_version_guard ON question_bank_item_version;
UPDATE question_bank_item_version SET bundle_hash=question_bank_bundle_hash(content,scoring);
ALTER TABLE question_bank_item_version ADD CHECK(bundle_hash ~ '^[0-9a-f]{64}$');

CREATE TABLE question_bank_review (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL, version_id UUID NOT NULL,
 reviewer_id UUID NOT NULL, decision TEXT NOT NULL CHECK(decision IN ('submit-review','approve','return-to-draft','publish')),
 comment TEXT NOT NULL DEFAULT '' CHECK(length(comment)<=2000), content_revision BIGINT NOT NULL, bundle_hash TEXT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), UNIQUE(tenant_id,id),
 FOREIGN KEY(tenant_id,version_id) REFERENCES question_bank_item_version(tenant_id,id),
 FOREIGN KEY(tenant_id,reviewer_id) REFERENCES app_user(tenant_id,id)
);
ALTER TABLE question_bank_item_version ADD FOREIGN KEY(tenant_id,last_review_id) REFERENCES question_bank_review(tenant_id,id);
CREATE TABLE question_bank_answer_version (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),tenant_id UUID NOT NULL,version_id UUID NOT NULL,content_revision BIGINT NOT NULL,facts JSONB NOT NULL,
 UNIQUE(tenant_id,version_id,content_revision),FOREIGN KEY(tenant_id,version_id) REFERENCES question_bank_item_version(tenant_id,id)
);
CREATE TABLE question_bank_rubric_version (LIKE question_bank_answer_version INCLUDING ALL);
ALTER TABLE question_bank_rubric_version ADD FOREIGN KEY(tenant_id,version_id) REFERENCES question_bank_item_version(tenant_id,id);
CREATE TABLE question_bank_item_asset (
 tenant_id UUID NOT NULL,version_id UUID NOT NULL,content_revision BIGINT NOT NULL,position INT NOT NULL,file_asset_id UUID NOT NULL,sha256 TEXT NOT NULL,facts JSONB NOT NULL,
 PRIMARY KEY(tenant_id,version_id,content_revision,position),FOREIGN KEY(tenant_id,version_id) REFERENCES question_bank_item_version(tenant_id,id),
 FOREIGN KEY(tenant_id,file_asset_id) REFERENCES file_asset(tenant_id,id)
);
CREATE TABLE question_bank_rubric_template (
 tenant_id UUID NOT NULL,template_id UUID NOT NULL,PRIMARY KEY(tenant_id,template_id),FOREIGN KEY(tenant_id,template_id) REFERENCES question_bank_item(tenant_id,id)
);
CREATE TABLE question_bank_rubric_template_version (
 tenant_id UUID NOT NULL,template_id UUID NOT NULL,version_id UUID NOT NULL,template_version INT NOT NULL,
 PRIMARY KEY(tenant_id,version_id),UNIQUE(tenant_id,template_id,template_version),
 FOREIGN KEY(tenant_id,template_id) REFERENCES question_bank_rubric_template(tenant_id,template_id),
 FOREIGN KEY(tenant_id,template_id,version_id) REFERENCES question_bank_item_version(tenant_id,item_id,id)
);
CREATE FUNCTION question_bank_append_only() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_OP<>'INSERT' OR pg_trigger_depth()<2 THEN RAISE EXCEPTION 'question bank child facts are append-only' USING ERRCODE='23514'; END IF; RETURN NEW;
END $$;
CREATE FUNCTION question_bank_review_guard() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE v question_bank_item_version; b UUID; a TEXT;
BEGIN
 IF TG_OP<>'INSERT' THEN RAISE EXCEPTION 'review records are immutable' USING ERRCODE='23514'; END IF;
 SELECT * INTO v FROM question_bank_item_version WHERE tenant_id=NEW.tenant_id AND id=NEW.version_id FOR UPDATE;
 SELECT bank_id INTO b FROM question_bank_item WHERE tenant_id=v.tenant_id AND id=v.item_id;
 a:=CASE NEW.decision WHEN 'approve' THEN 'review' WHEN 'publish' THEN 'publish' WHEN 'submit-review' THEN 'edit' ELSE 'review' END;
 IF NEW.decision='return-to-draft' AND NEW.reviewer_id=v.author_id THEN a:='edit'; END IF;
 IF NEW.content_revision<>v.revision OR NEW.bundle_hash<>v.bundle_hash
 OR NOT EXISTS(SELECT 1 FROM question_bank_acl WHERE tenant_id=NEW.tenant_id AND bank_id=b AND user_id=NEW.reviewer_id AND action=a)
 OR NOT EXISTS(SELECT 1 FROM question_bank_acl WHERE tenant_id=NEW.tenant_id AND bank_id=b AND user_id=NEW.reviewer_id AND action='read')
 OR (NEW.decision='approve' AND NEW.reviewer_id=v.author_id)
 OR NOT ((NEW.decision='submit-review' AND v.workflow_status='draft') OR (NEW.decision='approve' AND v.workflow_status='reviewing') OR (NEW.decision='publish' AND v.workflow_status='approved') OR (NEW.decision='return-to-draft' AND v.workflow_status IN ('reviewing','approved')))
 THEN RAISE EXCEPTION 'review is not bound to permitted current content' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER question_bank_review_guard BEFORE INSERT OR UPDATE OR DELETE ON question_bank_review FOR EACH ROW EXECUTE FUNCTION question_bank_review_guard();
CREATE OR REPLACE FUNCTION guard_question_bank_version_update() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE r question_bank_review; wanted TEXT;
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'versions must be retained' USING ERRCODE='23514'; END IF;
 IF TG_OP='INSERT' THEN
  IF NEW.workflow_status<>'draft' OR NEW.last_review_id IS NOT NULL THEN RAISE EXCEPTION 'new versions must be drafts' USING ERRCODE='23514'; END IF;
 ELSE
  IF (NEW.id,NEW.tenant_id,NEW.item_id,NEW.version_no,NEW.schema_version,NEW.source_version_id,NEW.author_id,NEW.created_at)
   IS DISTINCT FROM (OLD.id,OLD.tenant_id,OLD.item_id,OLD.version_no,OLD.schema_version,OLD.source_version_id,OLD.author_id,OLD.created_at)
  THEN RAISE EXCEPTION 'version identity is immutable' USING ERRCODE='23514'; END IF;
  IF NEW.workflow_status=OLD.workflow_status THEN
   IF OLD.workflow_status<>'draft' OR NEW.revision<>OLD.revision+1 OR NEW.last_review_id IS DISTINCT FROM OLD.last_review_id THEN RAISE EXCEPTION 'content is locked or revision invalid' USING ERRCODE='23514'; END IF;
  ELSE
   IF (NEW.content,NEW.scoring,NEW.content_hash) IS DISTINCT FROM (OLD.content,OLD.scoring,OLD.content_hash) THEN RAISE EXCEPTION 'workflow cannot change facts' USING ERRCODE='23514'; END IF;
   wanted:=CASE WHEN OLD.workflow_status='draft' AND NEW.workflow_status='reviewing' THEN 'submit-review'
    WHEN OLD.workflow_status='reviewing' AND NEW.workflow_status='approved' THEN 'approve'
    WHEN OLD.workflow_status='approved' AND NEW.workflow_status='published' THEN 'publish'
    WHEN OLD.workflow_status IN ('reviewing','approved') AND NEW.workflow_status='draft' THEN 'return-to-draft' ELSE '' END;
   SELECT * INTO r FROM question_bank_review WHERE tenant_id=NEW.tenant_id AND id=NEW.last_review_id AND version_id=NEW.id;
   IF wanted='' OR r.id IS NULL OR NEW.last_review_id IS NOT DISTINCT FROM OLD.last_review_id OR r.decision<>wanted OR r.content_revision<>OLD.revision OR r.bundle_hash<>OLD.bundle_hash
    OR NEW.revision<>OLD.revision+(CASE WHEN wanted='return-to-draft' THEN 1 ELSE 0 END)
   THEN RAISE EXCEPTION 'current content decision required' USING ERRCODE='23514'; END IF;
   IF wanted='publish' AND NOT EXISTS(SELECT 1 FROM question_bank_review WHERE tenant_id=OLD.tenant_id AND id=OLD.last_review_id AND decision='approve' AND content_revision=OLD.revision AND bundle_hash=OLD.bundle_hash AND reviewer_id<>OLD.author_id)
   THEN RAISE EXCEPTION 'approval required' USING ERRCODE='23514'; END IF;
  END IF;
 END IF;
 NEW.bundle_hash:=question_bank_bundle_hash(NEW.content,NEW.scoring);
 RETURN NEW;
END $$;
CREATE TRIGGER question_bank_version_guard BEFORE INSERT OR UPDATE OR DELETE ON question_bank_item_version FOR EACH ROW EXECUTE FUNCTION guard_question_bank_version_update();
CREATE FUNCTION question_bank_scoring_children() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE x JSONB; n INT:=0;
BEGIN
 IF TG_OP='UPDATE' AND NEW.revision=OLD.revision THEN RETURN NEW; END IF;
 IF NEW.scoring->'answer'<>'null'::jsonb THEN INSERT INTO question_bank_answer_version(tenant_id,version_id,content_revision,facts) VALUES(NEW.tenant_id,NEW.id,NEW.revision,NEW.scoring->'answer'); END IF;
 IF NEW.scoring->'rubric'<>'null'::jsonb THEN INSERT INTO question_bank_rubric_version(tenant_id,version_id,content_revision,facts) VALUES(NEW.tenant_id,NEW.id,NEW.revision,NEW.scoring->'rubric'); END IF;
 FOR x IN SELECT * FROM jsonb_array_elements(NEW.scoring->'assets') LOOP
  PERFORM 1 FROM file_asset WHERE tenant_id=NEW.tenant_id AND id=(x->>'file_asset_id')::uuid AND lifecycle_status='active' AND deleted_at IS NULL AND hash_sha256=x->>'sha256' AND original_name=x->>'name' AND content_type=x->>'content_type' FOR SHARE;
  IF NOT FOUND THEN RAISE EXCEPTION 'active asset facts required' USING ERRCODE='23514'; END IF;
  INSERT INTO question_bank_item_asset VALUES(NEW.tenant_id,NEW.id,NEW.revision,n,(x->>'file_asset_id')::uuid,x->>'sha256',x); n:=n+1;
 END LOOP;
 IF TG_OP='INSERT' AND EXISTS(SELECT 1 FROM question_bank_item WHERE tenant_id=NEW.tenant_id AND id=NEW.item_id AND kind='rubric_template') THEN
  INSERT INTO question_bank_rubric_template VALUES(NEW.tenant_id,NEW.item_id) ON CONFLICT DO NOTHING;
  INSERT INTO question_bank_rubric_template_version VALUES(NEW.tenant_id,NEW.item_id,NEW.id,NEW.version_no);
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER question_bank_scoring_children AFTER INSERT OR UPDATE ON question_bank_item_version FOR EACH ROW EXECUTE FUNCTION question_bank_scoring_children();
DO $$ DECLARE t TEXT; BEGIN
 FOREACH t IN ARRAY ARRAY['question_bank_answer_version','question_bank_rubric_version','question_bank_item_asset','question_bank_rubric_template','question_bank_rubric_template_version'] LOOP
  EXECUTE format('CREATE TRIGGER append_only BEFORE INSERT OR UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION question_bank_append_only()',t);
 END LOOP;
 FOREACH t IN ARRAY ARRAY['question_bank_answer_version','question_bank_rubric_version','question_bank_item_asset','question_bank_rubric_template','question_bank_rubric_template_version','question_bank_review'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t); EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('CREATE POLICY edugrade_tenant_isolation ON %I FOR ALL USING(edugrade_tenant_matches(tenant_id)) WITH CHECK(edugrade_tenant_matches(tenant_id))',t);
 END LOOP;
END $$;
CREATE FUNCTION question_bank_current_guard() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.current_published_version_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM question_bank_item_version WHERE tenant_id=NEW.tenant_id AND item_id=NEW.id AND id=NEW.current_published_version_id AND workflow_status='published') THEN RAISE EXCEPTION 'published current version required' USING ERRCODE='23514'; END IF; RETURN NEW;
END $$;
CREATE TRIGGER question_bank_current_guard BEFORE INSERT OR UPDATE ON question_bank_item FOR EACH ROW EXECUTE FUNCTION question_bank_current_guard();

ALTER TABLE question ADD COLUMN source_type TEXT NOT NULL DEFAULT 'manual' CHECK(source_type IN ('manual','question_bank'));
ALTER TABLE question ADD COLUMN assessment_archetype TEXT CHECK(assessment_archetype IN ('selected_response','exact_text','numeric_expression','structured_steps','short_constructed','extended_response','diagram_graph','table_experiment'));
ALTER TABLE question ADD COLUMN source_bank_item_id UUID;
ALTER TABLE question ADD COLUMN source_bank_item_version_id UUID;
ALTER TABLE question ADD COLUMN source_content_hash TEXT;
ALTER TABLE question ADD COLUMN bank_content JSONB;
ALTER TABLE question ADD FOREIGN KEY(tenant_id,source_bank_item_id,source_bank_item_version_id) REFERENCES question_bank_item_version(tenant_id,item_id,id);
ALTER TABLE question ADD CHECK((source_type='manual' AND source_bank_item_id IS NULL AND source_bank_item_version_id IS NULL AND source_content_hash IS NULL AND bank_content IS NULL) OR (source_type='question_bank' AND source_bank_item_id IS NOT NULL AND source_bank_item_version_id IS NOT NULL AND source_content_hash ~ '^[0-9a-f]{64}$' AND bank_content IS NOT NULL));
CREATE FUNCTION question_bank_source_guard() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF (NEW.source_type,NEW.source_bank_item_id,NEW.source_bank_item_version_id,NEW.source_content_hash,NEW.bank_content) IS DISTINCT FROM (OLD.source_type,OLD.source_bank_item_id,OLD.source_bank_item_version_id,OLD.source_content_hash,OLD.bank_content) THEN RAISE EXCEPTION 'question source is immutable' USING ERRCODE='23514'; END IF; RETURN NEW;
END $$;
CREATE TRIGGER question_bank_source_guard BEFORE UPDATE ON question FOR EACH ROW EXECUTE FUNCTION question_bank_source_guard();
-- A durable reference pins object identity and prevents deletion even after a
-- bank is archived. It also protects the file reconciler's cleanup path.
CREATE FUNCTION question_bank_file_guard() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF EXISTS(SELECT 1 FROM question_bank_item_asset WHERE tenant_id=OLD.tenant_id AND file_asset_id=OLD.id) THEN
  IF TG_OP='DELETE' THEN RAISE EXCEPTION 'referenced bank asset retained' USING ERRCODE='23514'; END IF;
  IF NEW.deleted_at IS NOT NULL OR NEW.lifecycle_status IN ('pending_delete','delete_failed','deleted') OR (NEW.hash_sha256,NEW.storage_bucket,NEW.storage_key,NEW.size_bytes,NEW.original_name,NEW.content_type) IS DISTINCT FROM (OLD.hash_sha256,OLD.storage_bucket,OLD.storage_key,OLD.size_bytes,OLD.original_name,OLD.content_type) THEN RAISE EXCEPTION 'referenced bank asset retained' USING ERRCODE='23514'; END IF;
 END IF; IF TG_OP='DELETE' THEN RETURN OLD; END IF; RETURN NEW;
END $$;
CREATE TRIGGER question_bank_file_guard BEFORE UPDATE OR DELETE ON file_asset FOR EACH ROW EXECUTE FUNCTION question_bank_file_guard();

INSERT INTO permission(tenant_id,code,name,resource,action,description)
SELECT t.id,'question_bank:'||a.action,a.name,'question_bank',a.action,'题库审核与发布仍需显式用户绑定' FROM tenant t CROSS JOIN (VALUES('review','审核题库版本'),('publish','发布题库版本')) a(action,name) WHERE t.deleted_at IS NULL ON CONFLICT(tenant_id,code) DO NOTHING;
INSERT INTO role_permission(tenant_id,role_id,permission_id)
SELECT r.tenant_id,r.id,p.id FROM role r JOIN permission p ON p.tenant_id=r.tenant_id WHERE r.code IN ('tenant_admin','school_admin','teacher') AND r.deleted_at IS NULL AND p.code IN ('question_bank:review','question_bank:publish') ON CONFLICT(tenant_id,role_id,permission_id) DO UPDATE SET deleted_at=NULL,updated_at=now();

-- Schema 1 stays byte-for-byte interpretable. Only bank-origin snapshots use
-- schema 2 and include source facts in their own target hash.
ALTER TABLE exam_question_snapshot ADD COLUMN source_snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER FUNCTION assessment_freeze_question_snapshot(UUID,UUID,UUID) RENAME TO assessment_freeze_question_snapshot_v1;
CREATE OR REPLACE FUNCTION assessment_freeze_bank_question_snapshot(
  p_tenant_id UUID,
  p_exam_id UUID,
  p_question_id UUID
)
RETURNS UUID
LANGUAGE plpgsql
AS $$
DECLARE
  config_record RECORD;
  rubric_snapshot JSONB;
  profile_snapshot JSONB;
  archetype_snapshot JSONB;
  snapshot_hash TEXT;
  snapshot_id UUID;
  source_snapshot JSONB;
BEGIN
  SELECT config.*, profile.code AS profile_code, profile.education_stage,
         profile.subject_code, profile.version AS profile_version,
         profile.parser_policy_json, profile.evidence_policy_json,
         profile.scoring_default_json, archetype.response_schema_json,
         archetype.evidence_types_json, archetype.default_scoring_mode
  INTO config_record
  FROM question_assessment_config config
  JOIN subject_profile profile
    ON profile.tenant_id = config.tenant_id AND profile.id = config.subject_profile_id
  JOIN question_archetype archetype ON archetype.code = config.archetype_code
  JOIN question
    ON question.tenant_id = config.tenant_id AND question.id = config.question_id
   AND question.exam_id = config.exam_id AND question.deleted_at IS NULL
  WHERE config.tenant_id = p_tenant_id
    AND config.exam_id = p_exam_id
    AND config.question_id = p_question_id;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING ERRCODE = 'P0001', MESSAGE = 'assessment profile missing for question';
  END IF;

  SELECT jsonb_build_object(
           'id', rubric.id,
           'version_id', rubric_version.id,
           'version', rubric_version.version,
           'status', rubric.status,
           'max_score', rubric.max_score,
           'points', rubric.points,
           'deductions', rubric.deductions,
           'examples', rubric.examples,
           'content_hash', rubric_version.content_hash
         )
  INTO rubric_snapshot
  FROM question_rubric rubric
  JOIN rubric_version
    ON rubric_version.tenant_id = rubric.tenant_id
   AND rubric_version.id = rubric.rubric_version_id
  WHERE rubric.tenant_id = p_tenant_id
    AND rubric.question_id = p_question_id
    AND rubric.deleted_at IS NULL
    AND rubric_version.deleted_at IS NULL
  ORDER BY rubric.created_at DESC, rubric.id DESC
  LIMIT 1;

  rubric_snapshot := COALESCE(rubric_snapshot, '{}'::jsonb);
  profile_snapshot := jsonb_build_object(
    'id', config_record.subject_profile_id,
    'code', config_record.profile_code,
    'education_stage', config_record.education_stage,
    'subject_code', config_record.subject_code,
    'version', config_record.profile_version,
    'parser_policy', config_record.parser_policy_json,
    'evidence_policy', config_record.evidence_policy_json,
    'scoring_default', config_record.scoring_default_json
  );
  archetype_snapshot := jsonb_build_object(
    'code', config_record.archetype_code,
    'response_schema', config_record.response_schema_json,
    'evidence_types', config_record.evidence_types_json,
    'default_scoring_mode', config_record.default_scoring_mode
  );
  SELECT jsonb_build_object('source_type',source_type,'item_id',source_bank_item_id,'version_id',source_bank_item_version_id,'bundle_hash',source_content_hash,'bank_content',bank_content) INTO source_snapshot FROM question WHERE tenant_id=p_tenant_id AND id=p_question_id;
  snapshot_hash := encode(digest(
    profile_snapshot::text || archetype_snapshot::text || rubric_snapshot::text ||
    config_record.allowed_evidence_types::text || config_record.risk_tier ||
    config_record.scoring_policy_json::text || source_snapshot::text,
    'sha256'
  ), 'hex');

  INSERT INTO exam_question_snapshot (
    tenant_id, exam_id, question_id, snapshot_version,
    subject_profile_id, subject_profile_version, archetype_code,
    allowed_evidence_types, risk_tier, profile_snapshot_json,
    archetype_snapshot_json, rubric_snapshot_json,
    scoring_policy_snapshot_json, content_hash, source_snapshot_json
  )
  VALUES (
    p_tenant_id, p_exam_id, p_question_id, 2,
    config_record.subject_profile_id, config_record.profile_version,
    config_record.archetype_code, config_record.allowed_evidence_types,
    config_record.risk_tier, profile_snapshot, archetype_snapshot,
    rubric_snapshot, config_record.scoring_policy_json, snapshot_hash, source_snapshot
  )
  ON CONFLICT (tenant_id, exam_id, question_id, snapshot_version) DO NOTHING
  RETURNING id INTO snapshot_id;

  IF snapshot_id IS NULL THEN
    SELECT id INTO snapshot_id
    FROM exam_question_snapshot
    WHERE tenant_id = p_tenant_id AND exam_id = p_exam_id
      AND question_id = p_question_id AND snapshot_version = 2;
  END IF;
  RETURN snapshot_id;
END
$$;


CREATE FUNCTION assessment_freeze_question_snapshot(p_tenant_id UUID,p_exam_id UUID,p_question_id UUID) RETURNS UUID LANGUAGE plpgsql AS $$ BEGIN
 IF EXISTS(SELECT 1 FROM question WHERE tenant_id=p_tenant_id AND id=p_question_id AND source_type='question_bank') THEN RETURN assessment_freeze_bank_question_snapshot(p_tenant_id,p_exam_id,p_question_id); END IF;
 RETURN assessment_freeze_question_snapshot_v1(p_tenant_id,p_exam_id,p_question_id);
END $$;
