-- Controlled metadata definitions are immutable per version. Bank-local groups
-- resolve membership on every authorization check so revocation is immediate.
ALTER TABLE question_bank ADD COLUMN metadata_schema_version INTEGER NOT NULL DEFAULT 1 CHECK(metadata_schema_version > 0);
ALTER TABLE question_bank_item ADD COLUMN revision BIGINT NOT NULL DEFAULT 1 CHECK(revision > 0);
ALTER TABLE question_bank_item_version DROP CONSTRAINT question_bank_item_version_schema_version_check;
ALTER TABLE question_bank_item_version ADD CHECK(schema_version > 0);
ALTER TABLE question_bank_acl DROP CONSTRAINT question_bank_acl_action_check;
ALTER TABLE question_bank_acl ADD CHECK(action IN ('read','create','edit','review','publish','retire','statistics','manage'));

CREATE TABLE question_bank_metadata_schema (
 tenant_id UUID NOT NULL, bank_id UUID NOT NULL, version INTEGER NOT NULL CHECK(version > 0),
 fields JSONB NOT NULL DEFAULT '[]'::jsonb CHECK(jsonb_typeof(fields)='array' AND jsonb_array_length(fields)<=64),
 taxonomies JSONB NOT NULL DEFAULT '[]'::jsonb CHECK(jsonb_typeof(taxonomies)='array' AND jsonb_array_length(taxonomies)<=32),
 created_by UUID NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,bank_id,version),
 FOREIGN KEY(tenant_id,bank_id) REFERENCES question_bank(tenant_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES app_user(tenant_id,id)
);
INSERT INTO question_bank_metadata_schema(tenant_id,bank_id,version,created_by)
SELECT tenant_id,id,1,created_by FROM question_bank;
ALTER TABLE question_bank ADD FOREIGN KEY(tenant_id,id,metadata_schema_version)
 REFERENCES question_bank_metadata_schema(tenant_id,bank_id,version) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE question_bank_item_metadata (
 tenant_id UUID NOT NULL, bank_id UUID NOT NULL, item_id UUID NOT NULL, version_id UUID NOT NULL,
 content_revision BIGINT NOT NULL, schema_version INTEGER NOT NULL, values JSONB NOT NULL DEFAULT '{}'::jsonb CHECK(jsonb_typeof(values)='object'),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,version_id,content_revision),
 FOREIGN KEY(tenant_id,bank_id) REFERENCES question_bank(tenant_id,id),
 FOREIGN KEY(tenant_id,item_id,version_id) REFERENCES question_bank_item_version(tenant_id,item_id,id),
 FOREIGN KEY(tenant_id,bank_id,schema_version) REFERENCES question_bank_metadata_schema(tenant_id,bank_id,version)
);

CREATE TABLE question_bank_group (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL, bank_id UUID NOT NULL,
 name TEXT NOT NULL CHECK(length(btrim(name)) BETWEEN 1 AND 160), created_by UUID NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,id), UNIQUE(tenant_id,bank_id,id), UNIQUE(tenant_id,bank_id,name),
 FOREIGN KEY(tenant_id,bank_id) REFERENCES question_bank(tenant_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES app_user(tenant_id,id)
);
CREATE TABLE question_bank_group_member (
 tenant_id UUID NOT NULL, group_id UUID NOT NULL, user_id UUID NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,group_id,user_id),
 FOREIGN KEY(tenant_id,group_id) REFERENCES question_bank_group(tenant_id,id) ON DELETE CASCADE,
 FOREIGN KEY(tenant_id,user_id) REFERENCES app_user(tenant_id,id)
);
CREATE TABLE question_bank_group_acl (
 tenant_id UUID NOT NULL, bank_id UUID NOT NULL, group_id UUID NOT NULL,
 action TEXT NOT NULL CHECK(action IN ('read','create','edit','review','publish','retire','statistics','manage')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY(tenant_id,bank_id,group_id,action),
 FOREIGN KEY(tenant_id,bank_id,group_id) REFERENCES question_bank_group(tenant_id,bank_id,id) ON DELETE CASCADE
);

CREATE OR REPLACE FUNCTION question_bank_actor_has_action(p_tenant UUID,p_bank UUID,p_actor UUID,p_action TEXT)
RETURNS BOOLEAN LANGUAGE SQL STABLE AS $$
 SELECT EXISTS(SELECT 1 FROM question_bank_acl a WHERE a.tenant_id=p_tenant AND a.bank_id=p_bank AND a.user_id=p_actor AND a.action=p_action)
 OR EXISTS(SELECT 1 FROM question_bank_group_acl a JOIN question_bank_group_member m ON m.tenant_id=a.tenant_id AND m.group_id=a.group_id
           WHERE a.tenant_id=p_tenant AND a.bank_id=p_bank AND m.user_id=p_actor AND a.action=p_action)
$$;

CREATE OR REPLACE FUNCTION question_bank_review_guard() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE v question_bank_item_version; b UUID; a TEXT;
BEGIN
 IF TG_OP<>'INSERT' THEN RAISE EXCEPTION 'review records are immutable' USING ERRCODE='23514'; END IF;
 SELECT * INTO v FROM question_bank_item_version WHERE tenant_id=NEW.tenant_id AND id=NEW.version_id FOR UPDATE;
 SELECT bank_id INTO b FROM question_bank_item WHERE tenant_id=v.tenant_id AND id=v.item_id;
 a:=CASE NEW.decision WHEN 'approve' THEN 'review' WHEN 'publish' THEN 'publish' WHEN 'submit-review' THEN 'edit' ELSE 'review' END;
 IF NEW.decision='return-to-draft' AND NEW.reviewer_id=v.author_id THEN a:='edit'; END IF;
 IF NEW.content_revision<>v.revision OR NEW.bundle_hash<>v.bundle_hash
 OR NOT question_bank_actor_has_action(NEW.tenant_id,b,NEW.reviewer_id,a)
 OR NOT question_bank_actor_has_action(NEW.tenant_id,b,NEW.reviewer_id,'read')
 OR (NEW.decision='approve' AND NEW.reviewer_id=v.author_id)
 OR NOT ((NEW.decision='submit-review' AND v.workflow_status='draft') OR (NEW.decision='approve' AND v.workflow_status='reviewing') OR (NEW.decision='publish' AND v.workflow_status='approved') OR (NEW.decision='return-to-draft' AND v.workflow_status IN ('reviewing','approved')))
 THEN RAISE EXCEPTION 'review is not bound to permitted current content' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $$;

CREATE FUNCTION question_bank_metadata_children() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE b UUID;
BEGIN
	IF TG_OP='UPDATE' AND NEW.revision=OLD.revision THEN RETURN NEW; END IF;
 SELECT bank_id INTO b FROM question_bank_item WHERE tenant_id=NEW.tenant_id AND id=NEW.item_id;
 INSERT INTO question_bank_item_metadata(tenant_id,bank_id,item_id,version_id,content_revision,schema_version,values)
 VALUES(NEW.tenant_id,b,NEW.item_id,NEW.id,NEW.revision,NEW.schema_version,COALESCE(NEW.content->'custom_metadata','{}'::jsonb));
 RETURN NEW;
END $$;
CREATE TRIGGER question_bank_metadata_children AFTER INSERT OR UPDATE ON question_bank_item_version
 FOR EACH ROW EXECUTE FUNCTION question_bank_metadata_children();
INSERT INTO question_bank_item_metadata(tenant_id,bank_id,item_id,version_id,content_revision,schema_version,values,created_at)
SELECT v.tenant_id,i.bank_id,v.item_id,v.id,v.revision,v.schema_version,COALESCE(v.content->'custom_metadata','{}'::jsonb),v.created_at
FROM question_bank_item_version v JOIN question_bank_item i ON i.tenant_id=v.tenant_id AND i.id=v.item_id;

CREATE FUNCTION question_bank_schema_immutable() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 RAISE EXCEPTION 'metadata schema versions are immutable' USING ERRCODE='23514';
END $$;
CREATE TRIGGER question_bank_schema_immutable BEFORE UPDATE OR DELETE ON question_bank_metadata_schema
 FOR EACH ROW EXECUTE FUNCTION question_bank_schema_immutable();
CREATE TRIGGER question_bank_item_metadata_immutable BEFORE UPDATE OR DELETE ON question_bank_item_metadata
 FOR EACH ROW EXECUTE FUNCTION question_bank_schema_immutable();

CREATE INDEX question_bank_group_member_actor_idx ON question_bank_group_member(tenant_id,user_id,group_id);
CREATE INDEX question_bank_group_acl_action_idx ON question_bank_group_acl(tenant_id,bank_id,action,group_id);
CREATE INDEX question_bank_search_idx ON question_bank_item(tenant_id,bank_id,subject_code,status,id);
CREATE INDEX question_bank_version_search_idx ON question_bank_item_version(tenant_id,workflow_status,updated_at DESC,id);
CREATE INDEX question_bank_content_gin_idx ON question_bank_item_version USING GIN(content jsonb_path_ops);

DO $$ DECLARE table_name TEXT; BEGIN
 FOREACH table_name IN ARRAY ARRAY['question_bank_metadata_schema','question_bank_item_metadata','question_bank_group','question_bank_group_member','question_bank_group_acl'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',table_name);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',table_name);
  EXECUTE format('CREATE POLICY edugrade_tenant_isolation ON %I FOR ALL USING(edugrade_tenant_matches(tenant_id)) WITH CHECK(edugrade_tenant_matches(tenant_id))',table_name);
 END LOOP;
END $$;

INSERT INTO permission(tenant_id,code,name,resource,action,description)
SELECT t.id,'question_bank:'||a.action,a.name,'question_bank',a.action,'题库动作仍需显式题库授权'
FROM tenant t CROSS JOIN (VALUES('retire','退役题库内容'),('statistics','查看题库统计')) a(action,name)
WHERE t.deleted_at IS NULL ON CONFLICT(tenant_id,code) DO NOTHING;
INSERT INTO role_permission(tenant_id,role_id,permission_id)
SELECT r.tenant_id,r.id,p.id FROM role r JOIN permission p ON p.tenant_id=r.tenant_id
WHERE r.code IN ('tenant_admin','school_admin','teacher') AND r.deleted_at IS NULL AND p.code IN ('question_bank:retire','question_bank:statistics')
ON CONFLICT(tenant_id,role_id,permission_id) DO UPDATE SET deleted_at=NULL,updated_at=now();
