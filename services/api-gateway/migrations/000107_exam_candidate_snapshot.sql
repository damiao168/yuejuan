-- Freeze the identity and class placement used by an exam. Current student
-- enrollment may change later without rewriting historical exam rosters.
CREATE TABLE exam_candidate_snapshot (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL REFERENCES exam(id),
  student_id UUID NOT NULL REFERENCES student(id),
  student_no_snapshot TEXT NOT NULL,
  student_name_snapshot TEXT NOT NULL,
  grade_id_snapshot UUID NOT NULL,
  grade_name_snapshot TEXT NOT NULL,
  class_id_snapshot UUID NOT NULL,
  class_name_snapshot TEXT NOT NULL,
  snapshot_version INT NOT NULL DEFAULT 1,
  captured_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, exam_id, student_id),
  CHECK (snapshot_version > 0)
);

CREATE INDEX idx_exam_candidate_snapshot_exam
  ON exam_candidate_snapshot (tenant_id, exam_id, class_name_snapshot, student_no_snapshot);

CREATE INDEX idx_exam_candidate_snapshot_student
  ON exam_candidate_snapshot (tenant_id, student_id, exam_id);

-- Existing exams receive a one-time baseline from their current roster.
INSERT INTO exam_candidate_snapshot (
  tenant_id,exam_id,student_id,student_no_snapshot,student_name_snapshot,
  grade_id_snapshot,grade_name_snapshot,class_id_snapshot,class_name_snapshot
)
SELECT DISTINCT
  ec.tenant_id,ec.exam_id,st.id,st.student_no,st.name,
  cls.grade_id,grade.name,cls.id,cls.name
FROM exam_class ec
JOIN school_class cls
  ON cls.tenant_id=ec.tenant_id AND cls.id=ec.class_id AND cls.deleted_at IS NULL
JOIN grade
  ON grade.tenant_id=cls.tenant_id AND grade.id=cls.grade_id AND grade.deleted_at IS NULL
JOIN student st
  ON st.tenant_id=ec.tenant_id AND st.class_id=ec.class_id
  AND st.status='active' AND st.deleted_at IS NULL
WHERE ec.deleted_at IS NULL
ON CONFLICT (tenant_id,exam_id,student_id) DO NOTHING;
