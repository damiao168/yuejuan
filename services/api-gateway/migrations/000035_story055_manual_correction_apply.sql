ALTER TABLE page_registration_correction
  ADD COLUMN applied_registration_run_id UUID;

ALTER TABLE page_registration_correction
  ADD CONSTRAINT fk_registration_correction_applied_run
  FOREIGN KEY (tenant_id, applied_registration_run_id)
  REFERENCES page_registration_run(tenant_id, id);
