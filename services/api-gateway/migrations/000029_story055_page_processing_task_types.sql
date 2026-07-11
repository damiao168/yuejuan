ALTER TABLE agent_worker_task
  DROP CONSTRAINT chk_agent_worker_task_type;

ALTER TABLE agent_worker_task
  ADD CONSTRAINT chk_agent_worker_task_type
  CHECK (task_type IN (
    'ocr',
    'layout',
    'preprocess',
    'image_quality',
    'ai_grade',
    'evidence_verify',
    'report_generate',
    'export',
    'desktop_sync',
    'capture_file_decode',
    'page_registration',
    'answer_segment_crop'
  ));
