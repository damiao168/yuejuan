export interface WorkspaceStage {
  key: string;
  label: string;
  state: "pending" | "current" | "completed" | string;
  action_route: string;
  summary?: string;
}

export interface WorkspaceNotice {
  code: string;
  title: string;
  message: string;
  severity: "blocker" | "warning" | string;
  action_label?: string;
  action_route?: string;
}

export interface WorkspaceMetric {
  key: string;
  label: string;
  value: number | string;
  suffix?: string;
  helper?: string;
}
