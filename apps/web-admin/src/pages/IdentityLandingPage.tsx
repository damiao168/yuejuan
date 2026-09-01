import { Card, Result, Tag } from "antd";
import { ClipboardList, GraduationCap } from "lucide-react";
import type { SessionUser } from "../auth/session";
import type { ProductExperience } from "../router/experience";

export function IdentityLandingPage({ user, experience }: { user: SessionUser; experience: ProductExperience }) {
  const auditor = experience === "auditor";
  const title = auditor ? "审计工作台" : "学生端";
  const description = auditor
    ? "当前账号仅可查看授权范围内的审计记录，数据范围由服务端权限策略强制约束。"
    : "当前账号仅可查看本人授权的成绩与学习反馈，数据范围由服务端权限策略强制约束。";
  return (
    <div className="page-stack" style={{ maxWidth: 920, margin: "0 auto" }}>
      <Result
        icon={auditor ? <ClipboardList size={48} /> : <GraduationCap size={48} />}
        title={title}
        subTitle={description}
      />
      <Card>
        <div style={{ display: "flex", gap: 12, alignItems: "center", flexWrap: "wrap" }}>
          <Tag color="blue">{user.displayName || user.username}</Tag>
          <span>{user.tenant}{user.school && user.school !== user.tenant ? ` · ${user.school}` : ""}</span>
        </div>
        <p style={{ color: "#646a73", marginBottom: 0, marginTop: 16 }}>
          如需扩大访问范围，请联系机构管理员。前端不会自行推断或提升账号权限。
        </p>
      </Card>
    </div>
  );
}
