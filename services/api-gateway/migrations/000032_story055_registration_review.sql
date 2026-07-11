ALTER TABLE page_registration_run
  ADD COLUMN IF NOT EXISTS confirmed_by UUID,
  ADD COLUMN IF NOT EXISTS confirmed_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS confirmation_reason TEXT;

ALTER TABLE page_registration_run
  ADD CONSTRAINT fk_registration_confirmed_by_tenant
  FOREIGN KEY (tenant_id, confirmed_by) REFERENCES app_user(tenant_id, id);
