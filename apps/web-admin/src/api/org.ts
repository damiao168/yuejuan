import { apiClient } from "./client";
import { buildQueryString } from "./query";

export interface Tenant {
  id: string;
  name: string;
  code: string;
  status: string;
}

export interface CreateTenantPayload {
  name: string;
  code: string;
  admin_username: string;
  admin_display_name: string;
  admin_password: string;
}

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

export async function listTenants(filter: { q?: string; limit?: number; cursor?: string } = {}) {
  return apiClient.request<{ tenants: Tenant[]; next_cursor: string; has_more: boolean }>(`/api/v1/tenants${buildQueryString(filter)}`);
}

export async function createTenant(payload: CreateTenantPayload) {
  return apiClient.request<{ tenant: Tenant }>("/api/v1/tenants", {
    method: "POST",
    body: JSON.stringify({ ...payload, status: "active" })
  });
}

export async function updateTenantStatus(id: string, status: "active" | "disabled") {
  return apiClient.request<{ tenant: Tenant }>(`/api/v1/tenants/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify({ status })
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

export async function listStudents(filter: { classId?: string; ids?: string[]; q?: string; limit?: number; cursor?: string } = {}) {
  const query = buildQueryString({ class_id: filter.classId, ids: filter.ids, q: filter.q, limit: filter.limit, cursor: filter.cursor });
  return apiClient.request<{ students: Student[]; next_cursor: string; has_more: boolean }>(`/api/v1/students${query}`);
}

export async function createStudent(payload: Pick<Student, "school_id" | "class_id" | "student_no" | "name"> & Partial<Pick<Student, "gender" | "status">>) {
  return apiClient.request<{ student: Student }>("/api/v1/students", {
    method: "POST",
    body: JSON.stringify({ ...payload, status: payload.status ?? "active" })
  });
}

export async function updateStudentStatus(id: string, status: "active" | "inactive") {
  return apiClient.request<{ student: Student }>(`/api/v1/students/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify({ status })
  });
}

export async function importStudentsCSV(csv: string) {
  return apiClient.request<{ result: StudentImportResult }>("/api/v1/students/import-csv", {
    method: "POST",
    headers: { "Content-Type": "text/csv; charset=utf-8" },
    body: csv
  });
}
