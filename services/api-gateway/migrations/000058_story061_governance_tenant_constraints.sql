ALTER TABLE model_provider
  DROP CONSTRAINT model_provider_created_by_fkey,
  ADD CONSTRAINT fk_model_provider_creator_tenant
    FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id);

ALTER TABLE model_deployment
  DROP CONSTRAINT model_deployment_created_by_fkey,
  ADD CONSTRAINT fk_model_deployment_creator_tenant
    FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id);

ALTER TABLE tenant_model_policy
  DROP CONSTRAINT tenant_model_policy_created_by_fkey,
  ADD CONSTRAINT fk_tenant_model_policy_creator_tenant
    FOREIGN KEY (tenant_id, created_by) REFERENCES app_user(tenant_id, id);

ALTER TABLE model_call_fact
  DROP CONSTRAINT model_call_fact_answer_segment_id_fkey,
  DROP CONSTRAINT model_call_fact_question_id_fkey,
  ADD CONSTRAINT fk_model_call_fact_answer_segment_tenant
    FOREIGN KEY (tenant_id, answer_segment_id) REFERENCES answer_segment(tenant_id, id),
  ADD CONSTRAINT fk_model_call_fact_question_tenant
    FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id);
