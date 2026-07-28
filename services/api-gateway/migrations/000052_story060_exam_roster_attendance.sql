-- STORY-060: keep exam reconciliation roster-based while persisting explicit
-- attendance decisions. The roster itself remains the live intersection of
-- exam_class and active students, avoiding a second mutable copy of identity
-- data. Every manual attendance decision is tenant scoped and auditable.

CREATE TABLE IF NOT EXISTS exam_student_attendance (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  exam_id UUID NOT NULL REFERENCES exam(id),
  student_id UUID NOT NULL REFERENCES student(id),
  status TEXT NOT NULL DEFAULT 'expected',
  reason TEXT NOT NULL,
  marked_by UUID NOT NULL REFERENCES app_user(id),
  marked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CONSTRAINT ck_exam_student_attendance_status
    CHECK (status IN ('expected', 'absent')),
  CONSTRAINT ck_exam_student_attendance_reason
    CHECK (length(btrim(reason)) > 0),
  UNIQUE (tenant_id, exam_id, student_id)
);

CREATE INDEX IF NOT EXISTS idx_exam_student_attendance_exam
ON exam_student_attendance (tenant_id, exam_id, status)
WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_exam_student_attendance_student
ON exam_student_attendance (tenant_id, student_id, exam_id)
WHERE deleted_at IS NULL;
