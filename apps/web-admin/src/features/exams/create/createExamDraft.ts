import type { Grade } from "../../../api/org";
import type { BlueprintSectionDraft, CreateExamDraft, SubjectExamDraft } from "./types";

const profiles: Record<string, { totalScore: number; durationMinutes: number; sections: Array<Omit<BlueprintSectionDraft, "id">> }> = {
  chinese: { totalScore: 150, durationMinutes: 150, sections: [{ title: "基础与阅读", questionType: "single_choice", questionCount: 10, scorePerQuestion: 3 }, { title: "阅读与表达", questionType: "short_answer", questionCount: 6, scorePerQuestion: 10 }, { title: "写作", questionType: "essay", questionCount: 1, scorePerQuestion: 60 }] },
  math: { totalScore: 150, durationMinutes: 120, sections: [{ title: "客观题", questionType: "single_choice", questionCount: 10, scorePerQuestion: 5 }, { title: "解答题", questionType: "calculation", questionCount: 10, scorePerQuestion: 10 }] },
  english: { totalScore: 150, durationMinutes: 120, sections: [{ title: "客观题", questionType: "single_choice", questionCount: 15, scorePerQuestion: 5 }, { title: "语言运用与写作", questionType: "short_answer", questionCount: 5, scorePerQuestion: 15 }] }
};

const scienceProfile = { totalScore: 100, durationMinutes: 75, sections: [{ title: "客观题", questionType: "single_choice", questionCount: 10, scorePerQuestion: 4 }, { title: "主观题", questionType: "short_answer", questionCount: 6, scorePerQuestion: 10 }] };

function localId(prefix: string) {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

export function createSubjectDraft(subject: string): SubjectExamDraft {
  const profile = profiles[subject] ?? scienceProfile;
  return { subject, totalScore: profile.totalScore, durationMinutes: profile.durationMinutes, candidateRule: "all_selected_classes", classIds: [], sections: profile.sections.map((section) => ({ ...section, id: localId("section") })) };
}

export function createBlankSection(): BlueprintSectionDraft {
  return { id: localId("section"), title: "新分区", questionType: "short_answer", questionCount: 1, scorePerQuestion: 10 };
}

export function initialCreateExamDraft(schoolId = "", grade?: Grade): CreateExamDraft {
  return {
    schoolId,
    name: "",
    examType: "",
    gradeId: grade?.id ?? "",
    classIds: [],
    subjects: [],
    gradingMode: "ai_assisted",
    publishPolicy: "after_admin_approval",
    appealEnabled: true
  };
}

export function subjectScore(subject: SubjectExamDraft) {
  return subject.sections.reduce((total, section) => total + section.questionCount * section.scorePerQuestion, 0);
}
