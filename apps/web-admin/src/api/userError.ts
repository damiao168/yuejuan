export const DEFAULT_USER_ERROR_MESSAGE = "操作失败，请稍后重试。";
export const NETWORK_USER_ERROR_MESSAGE = "网络连接异常，请检查网络后重试。";

const apiErrorMessages: Record<string, string> = {
  invalid_request: "请检查必填项和填写格式。",
  exam_not_collecting: "当前考试尚未进入答卷采集阶段，请先完成考试准备并开始采集。",
  exam_not_found: "考试不存在或已被删除。",
  access_scope_missing: "当前账号没有访问这些数据的权限。",
  capture_not_found: "采集记录不存在或已被删除。",
  capture_invalid_input: "采集信息有误，请检查后重试。",
  capture_invalid_transition: "当前采集状态不能执行此操作，请刷新页面查看最新状态。",
  capture_revision_conflict: "采集记录已被其他操作更新，请刷新页面后重试。",
  capture_duplicate_file: "该文件已加入当前批次，无需重复导入。",
  capture_operation_failed: "采集操作失败，请稍后重试。",
  resource_version_conflict: "任务已被其他人更新，请刷新页面后重试。",
  operation_in_progress: "操作正在处理中，请稍后查看结果。",
  operation_outcome_unknown: "原操作结果暂时无法确认，请保留记录并稍后重试。",
  business_receipt_missing: "操作已收到成功响应，但业务结果记录异常，请联系管理员核对，暂勿重复提交。",
  ambiguous_command_identity: "同一操作标识关联了多个请求，请联系管理员核对，暂勿重复提交。",
  command_rejected: "原操作已被明确拒绝，请修改后重新提交。",
  csrf_validation_failed: "页面安全状态已失效，请刷新页面后重试。",
  idempotency_key_required: "本次操作缺少安全重试标识，请刷新页面后重试。",
  idempotency_key_reused_with_different_request: "这次操作内容已变化，请重新发起。",
  capability_unavailable: "自动处理暂时不可用，任务已保留，可稍后继续或转人工处理。",
  invalid_credentials: "学校代码、账号或密码不正确。",
  unauthenticated: "登录状态已失效，请重新登录。",
  forbidden: "当前账号没有执行此操作的权限。",
  organization_scope_forbidden: "所选学校或班级不在你的管理范围内，请重新选择。",
  invalid_parent_scope: "所选学校、年级或班级已不存在，请刷新后重新选择。",
  school_list_failed: "学校信息暂时无法加载，请刷新后重试。",
  academic_year_list_failed: "学年信息暂时无法加载，请刷新后重试。",
  grade_cohort_list_failed: "年级信息暂时无法加载，请刷新后重试。",
  grade_list_failed: "年级列表暂时无法加载，请刷新后重试。",
  class_list_failed: "班级列表暂时无法加载，请刷新后重试。",
  student_list_failed: "学生名单暂时无法加载，请刷新后重试。",
  student_enrollment_list_failed: "学生的班级记录暂时无法加载，请刷新后重试。",
  user_list_failed: "教师与阅卷人员名单暂时无法加载，请刷新后重试。",
  role_list_failed: "可选人员角色暂时无法加载，请刷新后重试。",
  school_code_conflict: "该机构代码已被使用，请更换代码。",
  grade_school_invalid: "所选学校已不存在，请刷新后重新选择。",
  invalid_education_stage: "请选择初中或高中学段。",
  class_code_conflict: "该年级中已存在相同的班级代码，请更换代码。",
  class_grade_invalid: "所选年级与学校不匹配，请重新选择。",
  student_no_conflict: "该学号已存在，请更换学号。",
  student_class_invalid: "所选班级与学校不匹配，请重新选择。",
  student_not_found: "该学生已不存在，请刷新学生名单。",
  student_transfer_target_invalid: "所选学生或目标班级已不存在，请刷新后重新选择。",
  username_exists: "该登录账号已被使用，请更换账号。",
  weak_password: "初始密码至少需要 12 位，并同时包含大写字母、小写字母、数字和符号。",
  role_not_assignable: "所选人员角色当前不可用，请重新选择。",
  role_assignment_forbidden: "当前账号不能创建该角色的人员。",
  invalid_role_binding: "所选角色与学校或班级不匹配，请重新选择。",
  invalid_teacher_binding: "所选人员不是在用的学科教师，请重新选择。",
  user_create_failed: "人员账号暂时无法创建，请稍后重试。",
  teacher_bind_failed: "教师与班级暂时无法关联，请稍后重试。",
  school_create_failed: "机构暂时无法保存，请稍后重试。",
  grade_create_failed: "年级暂时无法保存，请稍后重试。",
  class_create_failed: "班级暂时无法保存，请稍后重试。",
  student_create_failed: "学生暂时无法添加，请稍后重试。",
  student_transfer_failed: "学生暂时无法调整班级，请稍后重试。",
  request_body_too_large: "填写内容过长，请精简后重试。",
  file_too_large: "上传内容过大，请调整文件后重试。",
  unsupported_media_type: "文件格式不受支持，请更换文件后重试。"
};

const httpErrorMessages: Record<number, string> = {
  400: "提交内容有误，请检查后重试。",
  401: "登录状态已失效，请重新登录。",
  403: "当前账号没有执行此操作的权限。",
  404: "请求的内容不存在或已被删除。",
  409: "当前数据状态已发生变化，请刷新后重试。",
  413: "上传内容过大，请调整文件后重试。",
  415: "文件格式不受支持，请更换文件后重试。",
  422: "提交内容暂时无法处理，请检查设置后重试。",
  429: "操作过于频繁，请稍后再试。"
};

export function getApiErrorMessage(code: string | undefined, status: number | undefined): string {
  if (code && apiErrorMessages[code]) return apiErrorMessages[code];
  if (status && status >= 500 && status <= 599) return "系统暂时无法完成操作，请稍后重试。";
  if (status && httpErrorMessages[status]) return httpErrorMessages[status];
  return DEFAULT_USER_ERROR_MESSAGE;
}

export function getUserErrorMessage(error: unknown, fallback?: string): string {
  if (isApiClientError(error)) return getApiErrorMessage(error.code, error.status);
  if (isNetworkError(error)) return NETWORK_USER_ERROR_MESSAGE;
  if (isAbortError(error)) return "操作已取消，请重新发起。";
  if (error instanceof Error && /[\u3400-\u9fff]/u.test(error.message)) return error.message;
  return safeFallback(fallback);
}

export function getSafeUserText(value: unknown, fallback: string): string {
  return typeof value === "string" && /[\u3400-\u9fff]/u.test(value) ? value : fallback;
}

const paperImportMessageMap: Record<string, string> = {
  page_processing_failed: "页面处理失败",
  source_ocr_failed: "图片文字识别失败",
  paper_ocr_unavailable: "文字识别服务暂不可用",
  ai_parse_failed: "考试资料解析失败",
  source_text_unavailable: "资料文字读取失败",
  paper_import_no_sources: "已删除全部考试资料，请重新上传正确的资料"
};

/** 将试卷资料解析返回的机器错误码转换为可直接展示给教师的中文。 */
export function getPaperImportUserMessage(value: unknown, fallback = "考试资料识别失败，请检查资料后重试"): string {
  if (typeof value !== "string" || !value.trim()) return fallback;
  let message = value.trim().replace(/\bOCR\b/gi, "文字识别");
  for (const [code, translated] of Object.entries(paperImportMessageMap)) message = message.split(code).join(translated);
  message = message
    .replace(/扫描文档\s+文字识别\s+失败/gi, "扫描文档文字识别失败")
    .replace(/exam has no questions/gi, "试卷尚未配置题目")
    .replace(/question is missing question_no or question_type/gi, "题目缺少题号或题型")
    .replace(/question\s+([^\s]+)\s+is missing answer_area/gi, "第$1题缺少答题区域")
    .replace(/question total\s+[\d.]+\s+does not equal exam total\s+[\d.]+/gi, "题目总分与考试总分不一致");
  return /[\u3400-\u9fff]/u.test(message) ? message : fallback;
}

function isApiClientError(error: unknown): error is { status: number; code: string } {
  if (!error || typeof error !== "object") return false;
  const candidate = error as { name?: unknown; status?: unknown; code?: unknown };
  return candidate.name === "ApiClientError"
    && typeof candidate.status === "number"
    && typeof candidate.code === "string";
}

function isNetworkError(error: unknown): boolean {
  if (!(error instanceof TypeError)) return false;
  return /failed to fetch|fetch failed|network(?:error| request failed| disconnected)|load failed/i.test(error.message);
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === "AbortError";
}

function safeFallback(fallback: string | undefined): string {
  return fallback && /[\u3400-\u9fff]/u.test(fallback) ? fallback : DEFAULT_USER_ERROR_MESSAGE;
}
