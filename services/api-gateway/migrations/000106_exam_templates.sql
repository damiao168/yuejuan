CREATE TABLE exam_template (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  school_id UUID,
  code TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  education_stage TEXT NOT NULL,
  exam_type TEXT NOT NULL DEFAULT '',
  region TEXT,
  curriculum TEXT,
  version INT NOT NULL DEFAULT 1,
  source TEXT NOT NULL DEFAULT 'school',
  status TEXT NOT NULL DEFAULT 'active',
  is_recommended BOOLEAN NOT NULL DEFAULT false,
  valid_from DATE,
  valid_to DATE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, code, version),
  UNIQUE (tenant_id, id),
  CHECK (education_stage IN ('junior','senior')),
  CHECK (source IN ('system','regional','school')),
  CHECK (status IN ('draft','active','retired')),
  FOREIGN KEY (tenant_id, school_id) REFERENCES school(tenant_id, id)
);

CREATE TABLE exam_template_subject (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_template_id UUID NOT NULL,
  subject_code TEXT NOT NULL,
  total_score NUMERIC(8,2) NOT NULL,
  duration_minutes INT NOT NULL,
  candidate_rule TEXT NOT NULL DEFAULT 'all_selected_classes',
  sort_order INT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, exam_template_id, subject_code),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, exam_template_id) REFERENCES exam_template(tenant_id, id),
  CHECK (total_score > 0),
  CHECK (duration_minutes > 0)
);

CREATE TABLE exam_template_section (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_template_subject_id UUID NOT NULL,
  title TEXT NOT NULL,
  question_type TEXT NOT NULL,
  question_count INT NOT NULL,
  score_per_question NUMERIC(8,2) NOT NULL,
  sort_order INT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, exam_template_subject_id, sort_order),
  FOREIGN KEY (tenant_id, exam_template_subject_id) REFERENCES exam_template_subject(tenant_id, id),
  CHECK (question_count > 0),
  CHECK (score_per_question > 0)
);

ALTER TABLE exam_session
  ADD COLUMN exam_template_id UUID REFERENCES exam_template(id),
  ADD COLUMN exam_template_version INT CHECK (exam_template_version > 0);

CREATE INDEX idx_exam_template_lookup
  ON exam_template (tenant_id, education_stage, status, is_recommended DESC, version DESC)
  WHERE deleted_at IS NULL;

INSERT INTO exam_template (
  id,tenant_id,code,name,description,education_stage,version,source,status,is_recommended
) VALUES
  ('00000000-0000-0000-0000-000000000601', '00000000-0000-0000-0000-000000000001', 'system.senior.standard', '系统通用高中考试方案', '适合校内期中、期末和阶段考试，可在本场考试中继续修改。', 'senior', 1, 'system', 'active', true),
  ('00000000-0000-0000-0000-000000000602', '00000000-0000-0000-0000-000000000001', 'system.junior.standard', '系统通用初中考试方案', '适合初中校内阶段考试，可按学校实际科目和题型调整。', 'junior', 1, 'system', 'active', true)
ON CONFLICT (tenant_id,code,version) DO NOTHING;

WITH subject_seed(template_code,subject_code,total_score,duration_minutes,sort_order) AS (VALUES
  ('system.senior.standard','chinese',150,150,1),('system.senior.standard','math',150,120,2),('system.senior.standard','english',150,120,3),
  ('system.junior.standard','chinese',120,120,1),('system.junior.standard','math',120,120,2),('system.junior.standard','english',120,120,3)
)
INSERT INTO exam_template_subject (tenant_id,exam_template_id,subject_code,total_score,duration_minutes,candidate_rule,sort_order)
SELECT template.tenant_id,template.id,seed.subject_code,seed.total_score,seed.duration_minutes,'all_selected_classes',seed.sort_order
FROM subject_seed seed
JOIN exam_template template ON template.tenant_id='00000000-0000-0000-0000-000000000001' AND template.code=seed.template_code AND template.version=1
ON CONFLICT (tenant_id,exam_template_id,subject_code) DO NOTHING;

WITH section_seed(template_code,subject_code,title,question_type,question_count,score_per_question,sort_order) AS (VALUES
  ('system.senior.standard','chinese','基础与阅读','single_choice',10,3,1),('system.senior.standard','chinese','阅读与表达','short_answer',6,10,2),('system.senior.standard','chinese','写作','essay',1,60,3),
  ('system.senior.standard','math','单项选择','single_choice',8,5,1),('system.senior.standard','math','多项选择','multiple_choice',3,6,2),('system.senior.standard','math','填空','fill_blank',3,5,3),('system.senior.standard','math','解答','calculation',7,11,4),
  ('system.senior.standard','english','客观题','single_choice',20,4,1),('system.senior.standard','english','语言运用','short_answer',5,8,2),('system.senior.standard','english','写作','essay',1,30,3),
  ('system.junior.standard','chinese','基础与阅读','single_choice',8,5,1),('system.junior.standard','chinese','阅读与表达','short_answer',4,10,2),('system.junior.standard','chinese','写作','essay',1,40,3),
  ('system.junior.standard','math','客观题','single_choice',12,5,1),('system.junior.standard','math','解答题','calculation',6,10,2),
  ('system.junior.standard','english','客观题','single_choice',15,4,1),('system.junior.standard','english','语言运用','short_answer',4,10,2),('system.junior.standard','english','写作','essay',1,20,3)
)
INSERT INTO exam_template_section (tenant_id,exam_template_subject_id,title,question_type,question_count,score_per_question,sort_order)
SELECT subject.tenant_id,subject.id,seed.title,seed.question_type,seed.question_count,seed.score_per_question,seed.sort_order
FROM section_seed seed
JOIN exam_template template ON template.tenant_id='00000000-0000-0000-0000-000000000001' AND template.code=seed.template_code AND template.version=1
JOIN exam_template_subject subject ON subject.tenant_id=template.tenant_id AND subject.exam_template_id=template.id AND subject.subject_code=seed.subject_code
ON CONFLICT (tenant_id,exam_template_subject_id,sort_order) DO NOTHING;
