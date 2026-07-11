import { apiClient } from "./client";

export interface School {
  id: string;
  tenant_id: string;
  name: string;
  code: string;
  status: string;
}

export interface Grade {
  id: string;
  tenant_id: string;
  school_id: string;
  name: string;
  level_no: number;
  academic_year: string;
  status: string;
}

export interface SchoolClass {
  id: string;
  tenant_id: string;
  school_id: string;
  grade_id: string;
  name: string;
  code: string;
  status: string;
}

export interface Student {
  id: string;
  tenant_id: string;
  school_id: string;
  class_id: string;
  student_no: string;
  name: string;
  gender?: string;
  status: string;
}

export interface StudentImportError {
  row: number;
  message: string;
}

export interface StudentImportResult {
  created: number;
  errors: StudentImportError[];
}

export async function createSchool(payload: Pick<School, "name" | "code">) {
  return apiClient.request<{ school: School }>("/api/v1/schools", {
    method: "POST",
    body: JSON.stringify({ ...payload, status: "active" })
  });
}

export async function listSchools() {
  return apiClient.request<{ schools: School[] }>("/api/v1/schools");
}

export async function listGrades(schoolId?: string) {
  const query = schoolId ? `?school_id=${encodeURIComponent(schoolId)}` : "";
  return apiClient.request<{ grades: Grade[] }>(`/api/v1/grades${query}`);
}

export async function createGrade(payload: Pick<Grade, "school_id" | "name" | "level_no" | "academic_year">) {
  return apiClient.request<{ grade: Grade }>("/api/v1/grades", {
    method: "POST",
    body: JSON.stringify({ ...payload, status: "active" })
  });
}

export async function listClasses(gradeId?: string) {
  const query = gradeId ? `?grade_id=${encodeURIComponent(gradeId)}` : "";
  return apiClient.request<{ classes: SchoolClass[] }>(`/api/v1/classes${query}`);
}

export async function createClass(payload: Pick<SchoolClass, "school_id" | "grade_id" | "name" | "code">) {
  return apiClient.request<{ class: SchoolClass }>("/api/v1/classes", {
    method: "POST",
    body: JSON.stringify({ ...payload, status: "active" })
  });
}

export async function listStudents(classId?: string) {
  const query = classId ? `?class_id=${encodeURIComponent(classId)}` : "";
  return apiClient.request<{ students: Student[] }>(`/api/v1/students${query}`);
}

export async function importStudentsCSV(csv: string) {
  return apiClient.request<{ result: StudentImportResult }>("/api/v1/students/import-csv", {
    method: "POST",
    headers: { "Content-Type": "text/csv; charset=utf-8" },
    body: csv
  });
}
