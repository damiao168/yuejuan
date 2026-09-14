-- Security facts use the existing outbox. Publishing to its current log sink
-- does not satisfy independent account notification or consume this backlog.
ALTER TABLE event_outbox
  ADD CONSTRAINT event_outbox_tenant_event_unique UNIQUE (tenant_id, id);

-- Preserve the existing business-outbox visibility while isolating the new
-- security aggregate. The process-only maintenance scope still dispatches it.
ALTER TABLE event_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE event_outbox FORCE ROW LEVEL SECURITY;
CREATE POLICY edugrade_outbox_scope ON event_outbox FOR ALL
  USING (aggregate_type <> 'auth_mfa' OR edugrade_tenant_matches(tenant_id))
  WITH CHECK (aggregate_type <> 'auth_mfa' OR edugrade_tenant_matches(tenant_id));

-- Tenant-scoped request code must not rewrite or erase a security fact after
-- it has committed. The maintenance-only dispatcher may still update lease,
-- retry and publication columns; the notification intent remains independent.
CREATE OR REPLACE FUNCTION guard_auth_mfa_outbox_fact()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF (OLD.aggregate_type = 'auth_mfa'
      OR (TG_OP = 'UPDATE' AND NEW.aggregate_type = 'auth_mfa'))
     AND current_setting('edugrade.tenant_id', true) IS DISTINCT FROM 'maintenance' THEN
    RAISE EXCEPTION 'auth_mfa outbox facts are maintenance-only after insert';
  END IF;
  IF TG_OP = 'DELETE' THEN
    RETURN OLD;
  END IF;
  RETURN NEW;
END
$$;

CREATE TRIGGER trg_auth_mfa_outbox_fact_guard
BEFORE UPDATE OR DELETE ON event_outbox
FOR EACH ROW EXECUTE FUNCTION guard_auth_mfa_outbox_fact();

CREATE TABLE auth_security_notification_intent (
  event_id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  user_id UUID NOT NULL,
  status TEXT NOT NULL DEFAULT 'awaiting_channel' CHECK (status = 'awaiting_channel'),
  created_at TIMESTAMPTZ NOT NULL,
  FOREIGN KEY (tenant_id, event_id) REFERENCES event_outbox(tenant_id, id),
  FOREIGN KEY (tenant_id, user_id) REFERENCES app_user(tenant_id, id)
);
CREATE INDEX auth_security_notification_intent_pending_idx
  ON auth_security_notification_intent (tenant_id, created_at, event_id);

ALTER TABLE auth_security_notification_intent ENABLE ROW LEVEL SECURITY;
ALTER TABLE auth_security_notification_intent FORCE ROW LEVEL SECURITY;
CREATE POLICY edugrade_tenant_isolation ON auth_security_notification_intent FOR ALL
  USING (edugrade_tenant_matches(tenant_id))
  WITH CHECK (edugrade_tenant_matches(tenant_id));
GRANT SELECT, INSERT ON auth_security_notification_intent TO edugrade_tenant_runtime;
REVOKE UPDATE, DELETE ON auth_security_notification_intent FROM edugrade_tenant_runtime;

COMMENT ON TABLE auth_security_notification_intent IS
'Atomic MFA notification requirements, not delivered messages. No verified contacts or external delivery are implemented. Log publication cannot acknowledge these rows.';
