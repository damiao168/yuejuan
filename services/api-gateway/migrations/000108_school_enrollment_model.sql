ALTER TABLE school ADD COLUMN education_stages JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE TABLE academic_year (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  school_id UUID NOT NULL REFERENCES school(id),
  name TEXT NOT NULL,
  start_year INT NOT NULL,
  end_year INT NOT NULL,
  starts_at DATE NOT NULL,
  ends_at DATE NOT NULL,
  is_current BOOLEAN NOT NULL DEFAULT false,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, school_id, start_year),
  UNIQUE (tenant_id, id),
  CHECK (end_year = start_year + 1),
  CHECK (ends_at > starts_at)
);

CREATE UNIQUE INDEX uq_academic_year_current_school
  ON academic_year (tenant_id, school_id) WHERE is_current AND deleted_at IS NULL;

CREATE TABLE grade_cohort (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  school_id UUID NOT NULL REFERENCES school(id),
  education_stage TEXT NOT NULL,
  entry_year INT NOT NULL,
  expected_graduation_year INT NOT NULL,
  name TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, school_id, education_stage, entry_year),
  UNIQUE (tenant_id, id),
  CHECK (education_stage IN ('junior','senior')),
  CHECK (expected_graduation_year > entry_year)
);

WITH grade_year AS (
  SELECT DISTINCT tenant_id,school_id,
    CASE WHEN academic_year ~ '^[0-9]{4}-[0-9]{4}$' THEN split_part(academic_year,'-',1)::int ELSE EXTRACT(YEAR FROM CURRENT_DATE)::int END AS start_year
  FROM grade WHERE deleted_at IS NULL
)
INSERT INTO academic_year (tenant_id,school_id,name,start_year,end_year,starts_at,ends_at,is_current)
SELECT tenant_id,school_id,start_year::text || '-' || (start_year+1)::text || '学年',start_year,start_year+1,
  make_date(start_year,9,1),make_date(start_year+1,8,31),
  CURRENT_DATE BETWEEN make_date(start_year,9,1) AND make_date(start_year+1,8,31)
FROM grade_year
ON CONFLICT (tenant_id,school_id,start_year) DO NOTHING;

WITH cohort_seed AS (
  SELECT DISTINCT g.tenant_id,g.school_id,g.education_stage,
    ay.start_year - CASE WHEN g.education_stage='senior' THEN GREATEST(g.level_no-10,0) ELSE GREATEST(g.level_no-7,0) END AS entry_year
  FROM grade g JOIN academic_year ay
    ON ay.tenant_id=g.tenant_id AND ay.school_id=g.school_id
    AND ay.start_year=CASE WHEN g.academic_year ~ '^[0-9]{4}-[0-9]{4}$' THEN split_part(g.academic_year,'-',1)::int ELSE EXTRACT(YEAR FROM CURRENT_DATE)::int END
  WHERE g.deleted_at IS NULL
)
INSERT INTO grade_cohort (tenant_id,school_id,education_stage,entry_year,expected_graduation_year,name)
SELECT tenant_id,school_id,education_stage,entry_year,entry_year+3,entry_year::text || '级'
FROM cohort_seed
ON CONFLICT (tenant_id,school_id,education_stage,entry_year) DO NOTHING;

ALTER TABLE grade ADD COLUMN academic_year_id UUID REFERENCES academic_year(id);
ALTER TABLE grade ADD COLUMN grade_cohort_id UUID REFERENCES grade_cohort(id);

UPDATE grade g SET
  academic_year_id=ay.id,
  grade_cohort_id=cohort.id
FROM academic_year ay,grade_cohort cohort
WHERE ay.tenant_id=g.tenant_id AND ay.school_id=g.school_id
  AND ay.start_year=CASE WHEN g.academic_year ~ '^[0-9]{4}-[0-9]{4}$' THEN split_part(g.academic_year,'-',1)::int ELSE EXTRACT(YEAR FROM CURRENT_DATE)::int END
  AND cohort.tenant_id=g.tenant_id AND cohort.school_id=g.school_id
  AND cohort.education_stage=g.education_stage
  AND cohort.entry_year=ay.start_year-CASE WHEN g.education_stage='senior' THEN GREATEST(g.level_no-10,0) ELSE GREATEST(g.level_no-7,0) END;

ALTER TABLE grade ALTER COLUMN academic_year_id SET NOT NULL;
ALTER TABLE grade ALTER COLUMN grade_cohort_id SET NOT NULL;

ALTER TABLE school_class ADD COLUMN academic_year_id UUID REFERENCES academic_year(id);
ALTER TABLE school_class ADD COLUMN grade_cohort_id UUID REFERENCES grade_cohort(id);
ALTER TABLE school_class ADD COLUMN class_no INT;

UPDATE school_class cls SET academic_year_id=g.academic_year_id,grade_cohort_id=g.grade_cohort_id
FROM grade g WHERE g.tenant_id=cls.tenant_id AND g.id=cls.grade_id;
ALTER TABLE school_class ALTER COLUMN academic_year_id SET NOT NULL;
ALTER TABLE school_class ALTER COLUMN grade_cohort_id SET NOT NULL;

ALTER TABLE student ADD COLUMN admission_year INT;

CREATE TABLE student_enrollment (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenant(id),
  school_id UUID NOT NULL REFERENCES school(id),
  student_id UUID NOT NULL REFERENCES student(id),
  academic_year_id UUID NOT NULL REFERENCES academic_year(id),
  grade_cohort_id UUID NOT NULL REFERENCES grade_cohort(id),
  class_id UUID NOT NULL REFERENCES school_class(id),
  status TEXT NOT NULL DEFAULT 'enrolled',
  start_date DATE NOT NULL,
  end_date DATE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  CHECK (status IN ('enrolled','transferred','suspended','graduated','withdrawn')),
  CHECK (end_date IS NULL OR end_date >= start_date)
);

CREATE UNIQUE INDEX uq_student_enrollment_active_year
  ON student_enrollment (tenant_id,student_id,academic_year_id)
  WHERE status='enrolled' AND end_date IS NULL AND deleted_at IS NULL;

INSERT INTO student_enrollment (tenant_id,school_id,student_id,academic_year_id,grade_cohort_id,class_id,status,start_date)
SELECT st.tenant_id,st.school_id,st.id,cls.academic_year_id,cls.grade_cohort_id,cls.id,
  CASE WHEN st.status='active' THEN 'enrolled' ELSE 'withdrawn' END,ay.starts_at
FROM student st
JOIN school_class cls ON cls.tenant_id=st.tenant_id AND cls.id=st.class_id
JOIN academic_year ay ON ay.tenant_id=cls.tenant_id AND ay.id=cls.academic_year_id
WHERE st.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM student_enrollment existing
    WHERE existing.tenant_id=st.tenant_id AND existing.student_id=st.id
      AND existing.academic_year_id=cls.academic_year_id AND existing.status='enrolled'
      AND existing.end_date IS NULL AND existing.deleted_at IS NULL
  );

UPDATE student st SET admission_year=cohort.entry_year
FROM student_enrollment enrollment
JOIN grade_cohort cohort ON cohort.tenant_id=enrollment.tenant_id AND cohort.id=enrollment.grade_cohort_id
WHERE enrollment.tenant_id=st.tenant_id AND enrollment.student_id=st.id;

UPDATE school s SET education_stages=source.stages
FROM (
  SELECT school_id,jsonb_agg(DISTINCT education_stage ORDER BY education_stage) AS stages
  FROM grade_cohort WHERE deleted_at IS NULL GROUP BY school_id
) source WHERE source.school_id=s.id;

CREATE INDEX idx_enrollment_class ON student_enrollment (tenant_id,academic_year_id,class_id,student_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_enrollment_cohort ON student_enrollment (tenant_id,academic_year_id,grade_cohort_id,student_id) WHERE deleted_at IS NULL;
