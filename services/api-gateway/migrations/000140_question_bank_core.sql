-- Reusable content lives upstream of exam Question. This slice permits drafts only.
CREATE TABLE question_bank (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 tenant_id UUID NOT NULL REFERENCES tenant(id),
 school_id UUID NOT NULL,
 name TEXT NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 160),
 description TEXT NOT NULL DEFAULT '' CHECK (length(description)<=2000),
 status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived')),
 revision BIGINT NOT NULL DEFAULT 1 CHECK (revision>0),
 created_by UUID NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,id),
 FOREIGN KEY(tenant_id,school_id) REFERENCES school(tenant_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES app_user(tenant_id,id)
);
CREATE TABLE question_bank_acl (
 tenant_id UUID NOT NULL, bank_id UUID NOT NULL, user_id UUID NOT NULL,
 action TEXT NOT NULL CHECK(action IN ('read','create','edit','manage')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,bank_id,user_id,action),
 FOREIGN KEY(tenant_id,bank_id) REFERENCES question_bank(tenant_id,id),
 FOREIGN KEY(tenant_id,user_id) REFERENCES app_user(tenant_id,id)
);
CREATE TABLE question_bank_item (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL, bank_id UUID NOT NULL,
 item_code TEXT NOT NULL CHECK(item_code ~ '^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$'),
 subject_code TEXT NOT NULL CHECK(subject_code IN ('chinese','mathematics','english','physics','chemistry','biology','history','geography','ethics_politics')),
 grade_scope TEXT NOT NULL CHECK(length(btrim(grade_scope)) BETWEEN 1 AND 80),
 current_published_version_id UUID CHECK(current_published_version_id IS NULL),
 status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','retired','archived')),
 created_by UUID NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,id), UNIQUE(tenant_id,bank_id,item_code),
 FOREIGN KEY(tenant_id,bank_id) REFERENCES question_bank(tenant_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES app_user(tenant_id,id)
);
CREATE TABLE question_bank_item_version (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL, item_id UUID NOT NULL,
 version_no INTEGER NOT NULL CHECK(version_no>0), schema_version INTEGER NOT NULL DEFAULT 1 CHECK(schema_version=1),
 revision BIGINT NOT NULL DEFAULT 1 CHECK(revision>0),
 workflow_status TEXT NOT NULL DEFAULT 'draft' CHECK(workflow_status='draft'),
 source_version_id UUID, author_id UUID NOT NULL,
 content JSONB NOT NULL CHECK(jsonb_typeof(content)='object'),
 content_hash TEXT NOT NULL CHECK(content_hash ~ '^[0-9a-f]{64}$'),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,id), UNIQUE(tenant_id,item_id,id), UNIQUE(tenant_id,item_id,version_no),
 FOREIGN KEY(tenant_id,item_id) REFERENCES question_bank_item(tenant_id,id),
 FOREIGN KEY(tenant_id,author_id) REFERENCES app_user(tenant_id,id),
 FOREIGN KEY(tenant_id,item_id,source_version_id) REFERENCES question_bank_item_version(tenant_id,item_id,id),
 CHECK(jsonb_typeof(content->'default_score')='number' AND (content->>'default_score')::numeric>0 AND (content->>'default_score')::numeric<=100000 AND (content->>'default_score')::numeric=round((content->>'default_score')::numeric,2)),
 CHECK(content ?& ARRAY['question_type','assessment_archetype','stem','options','default_score','knowledge_points','metadata']),
 CHECK(length(btrim(content->>'stem')) BETWEEN 1 AND 50000),
 CHECK(jsonb_typeof(content->'options')='array' AND jsonb_array_length(content->'options')<=32),
 CHECK(jsonb_typeof(content->'knowledge_points')='array' AND jsonb_array_length(content->'knowledge_points')<=64),
 CHECK(content->>'question_type' IN ('single_choice','multiple_choice','true_false','fill_blank','numeric','formula','short_answer','calculation','essay','discussion','coding')),
 CHECK(content->>'assessment_archetype' IN ('selected_response','exact_text','numeric_expression','structured_steps','short_constructed','extended_response','diagram_graph','table_experiment')),
 CHECK((jsonb_typeof(content->'question_type')='string' AND jsonb_typeof(content->'assessment_archetype')='string'
  AND jsonb_typeof(content->'stem')='string' AND jsonb_typeof(content->'default_score')='number'
  AND jsonb_typeof(content->'options')='array' AND jsonb_typeof(content->'knowledge_points')='array'
  AND jsonb_typeof(content->'metadata')='object') IS TRUE),
 CHECK(NOT jsonb_path_exists(content,'$.options[*] ? (@.type() != "string")')
  AND NOT jsonb_path_exists(content,'$.knowledge_points[*] ? (@.type() != "string")')),
 CHECK(((content->'metadata'->>'subject_code') IN ('chinese','mathematics','english','physics','chemistry','biology','history','geography','ethics_politics')
  AND (content->'metadata'->>'education_stage') IN ('junior','senior')
  AND length(btrim(content->'metadata'->>'grade_scope')) BETWEEN 1 AND 80
  AND (content->'metadata'->>'difficulty_band') IN ('unclassified','easy','medium','hard')
  AND (content->'metadata'->>'cognitive_level') IN ('unclassified','remember','understand','apply','analyze','evaluate','create')
  AND (content->'metadata'->>'copyright') IN ('unknown','owned','licensed')
  AND (content->'metadata'->>'language') IN ('zh-CN','en')) IS TRUE)
);
ALTER TABLE question_bank_item ADD CONSTRAINT question_bank_current_version_fk
 FOREIGN KEY(tenant_id,id,current_published_version_id) REFERENCES question_bank_item_version(tenant_id,item_id,id);
CREATE INDEX question_bank_school_idx ON question_bank(tenant_id,school_id,id);
CREATE INDEX question_bank_acl_actor_idx ON question_bank_acl(tenant_id,user_id,action,bank_id);
CREATE INDEX question_bank_version_history_idx ON question_bank_item_version(tenant_id,item_id,version_no DESC);

CREATE OR REPLACE FUNCTION guard_question_bank_version_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'question bank versions must be retained'; END IF;
 IF OLD.workflow_status<>'draft' THEN RAISE EXCEPTION 'question bank version content is locked'; END IF;
 IF (NEW.id,NEW.tenant_id,NEW.item_id,NEW.version_no,NEW.schema_version,NEW.source_version_id,NEW.author_id,NEW.created_at)
  IS DISTINCT FROM (OLD.id,OLD.tenant_id,OLD.item_id,OLD.version_no,OLD.schema_version,OLD.source_version_id,OLD.author_id,OLD.created_at)
 THEN RAISE EXCEPTION 'question bank version identity is immutable'; END IF;
 IF NEW.revision<>OLD.revision+1 THEN RAISE EXCEPTION 'question bank revision must increment once'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER question_bank_version_guard BEFORE UPDATE OR DELETE ON question_bank_item_version
 FOR EACH ROW EXECUTE FUNCTION guard_question_bank_version_update();

DO $$ DECLARE table_name TEXT; BEGIN
 FOREACH table_name IN ARRAY ARRAY['question_bank','question_bank_acl','question_bank_item','question_bank_item_version'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',table_name);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',table_name);
  EXECUTE format('CREATE POLICY edugrade_tenant_isolation ON %I FOR ALL USING(edugrade_tenant_matches(tenant_id)) WITH CHECK(edugrade_tenant_matches(tenant_id))',table_name);
 END LOOP;
END $$;

INSERT INTO permission(tenant_id,code,name,resource,action,description)
SELECT t.id,'question_bank:'||a.action,a.name,'question_bank',a.action,'题库动作仍需显式题库内容授权'
FROM tenant t CROSS JOIN (VALUES ('read','浏览题库'),('create','创建题库内容'),('edit','编辑题库草稿'),('manage','管理题库结构')) a(action,name)
-- The platform tenant is also the provisioning template for future tenants.
-- No platform_admin assignment is added; platform actors are rejected by the API.
WHERE t.deleted_at IS NULL
ON CONFLICT(tenant_id,code) DO NOTHING;
INSERT INTO role_permission(tenant_id,role_id,permission_id)
SELECT r.tenant_id,r.id,p.id FROM role r JOIN permission p ON p.tenant_id=r.tenant_id
WHERE r.code IN ('tenant_admin','school_admin','teacher') AND r.deleted_at IS NULL AND p.code IN ('question_bank:read','question_bank:create','question_bank:edit')
ON CONFLICT(tenant_id,role_id,permission_id) DO UPDATE SET deleted_at=NULL,updated_at=now();
INSERT INTO role_permission(tenant_id,role_id,permission_id)
SELECT r.tenant_id,r.id,p.id FROM role r JOIN permission p ON p.tenant_id=r.tenant_id
WHERE r.code IN ('tenant_admin','school_admin','teacher') AND r.deleted_at IS NULL AND p.code='question_bank:manage'
ON CONFLICT(tenant_id,role_id,permission_id) DO UPDATE SET deleted_at=NULL,updated_at=now();
