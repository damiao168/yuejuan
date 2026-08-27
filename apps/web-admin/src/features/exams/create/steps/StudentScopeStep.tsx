import { useState } from "react";
import { Button, Checkbox, Input, Select } from "antd";
import type { Grade, SchoolClass } from "../../../../api/org";
import type { CreateExamDraft } from "../types";

export function StudentScopeStep({ draft, grades, classes, studentCountByClass, onChange }: { draft: CreateExamDraft; grades: Grade[]; classes: SchoolClass[]; studentCountByClass: Map<string, number>; onChange: (patch: Partial<CreateExamDraft>) => void }) {
  const [query, setQuery] = useState("");
  const available = classes.filter((item) => item.grade_id === draft.gradeId && item.status === "active");
  const visible = available.filter((item) => `${item.name} ${item.code}`.toLowerCase().includes(query.trim().toLowerCase()));
  const selected = new Set(draft.classIds);
  const allSelected = available.length > 0 && available.every((item) => selected.has(item.id));
  const toggle = (classId: string) => onChange({ classIds: selected.has(classId) ? draft.classIds.filter((id) => id !== classId) : [...draft.classIds, classId] });
  const countsReliable = draft.classIds.length > 0 && draft.classIds.every((classId) => studentCountByClass.has(classId));
  const studentCount = draft.classIds.reduce((total, classId) => total + (studentCountByClass.get(classId) ?? 0), 0);
  return (
    <div className="exam-scope-step">
      <label className="exam-create-field"><span>年级 *</span><Select value={draft.gradeId || undefined} placeholder="选择年级" options={grades.map((grade) => ({ label: `${grade.name} · ${grade.academic_year}`, value: grade.id }))} onChange={(gradeId) => onChange({ gradeId, classIds: [] })} /></label>
      <div className="exam-scope-toolbar"><span>参考班级 *</span><Button type="link" disabled={!available.length} onClick={() => onChange({ classIds: allSelected ? [] : available.map((item) => item.id) })}>{allSelected ? "取消全选" : "全选当前年级"}</Button></div>
      {available.length > 12 ? <Input allowClear value={query} placeholder="搜索班级" onChange={(event) => setQuery(event.target.value)} /> : null}
      <div className="exam-class-list">
        {visible.map((schoolClass) => (
          <button type="button" key={schoolClass.id} className={selected.has(schoolClass.id) ? "selected" : ""} onClick={() => toggle(schoolClass.id)}>
            <Checkbox checked={selected.has(schoolClass.id)} tabIndex={-1} />
            <span><strong>{schoolClass.name}</strong><small>{schoolClass.code}</small></span>
            <em>{studentCountByClass.has(schoolClass.id) ? `${studentCountByClass.get(schoolClass.id)} 人` : ""}</em>
          </button>
        ))}
        {!available.length ? <div className="exam-create-inline-empty">当前年级暂无可用班级，请先到成员管理完成班级设置。</div> : null}
      </div>
      <div className="exam-scope-total">已选择 <strong>{draft.classIds.length}</strong> 个班级{countsReliable ? <> · <strong>{studentCount}</strong> 名学生</> : null}</div>
    </div>
  );
}
