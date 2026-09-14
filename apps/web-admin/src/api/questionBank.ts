import { EduGradeApi } from "@edugrade/sdk";
import { apiClient } from "./client";

export const questionBankApi = new EduGradeApi(apiClient);
