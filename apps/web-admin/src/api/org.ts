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

export async function listSchools() {
  return apiClient.request<{ schools: School[] }>("/api/v1/schools");
}

export async function listGrades(schoolId?: string) {
  const query = schoolId ? `?school_id=${encodeURIComponent(schoolId)}` : "";
  return apiClient.request<{ grades: Grade[] }>(`/api/v1/grades${query}`);
}

export async function listClasses(gradeId?: string) {
  const query = gradeId ? `?grade_id=${encodeURIComponent(gradeId)}` : "";
  return apiClient.request<{ classes: SchoolClass[] }>(`/api/v1/classes${query}`);
}

export async function listStudents(classId?: string) {
  const query = classId ? `?class_id=${encodeURIComponent(classId)}` : "";
  return apiClient.request<{ students: Student[] }>(`/api/v1/students${query}`);
}
