import { Button, Tag } from "antd";
import { ClipboardList, GraduationCap } from "lucide-react";
import type { SessionUser } from "../auth/session";
import type { ProductExperience } from "../router/experience";

export function IdentityLandingPage({ user, experience, onNavigate }: { user: SessionUser; experience: ProductExperience; onNavigate: (path: string) => void }) {
  const auditor = experience === "auditor";
  const title = auditor ? "审计工作台" : "学生端";
  const description = auditor
    ? "当前账号仅可查看授权范围内的审计记录，数据范围由服务端权限策略强制约束。"
    : "当前账号仅可查看本人授权的成绩与学习反馈，数据范围由服务端权限策略强制约束。";
  return (
    <div className="page-stack identity-workspace">
      <header className="identity-heading">{auditor ? <ClipboardList size={28} /> : <GraduationCap size={28} />}<div><h1>{title}</h1><p>{description}</p></div></header>
      <section>
        <div style={{ display: "flex", gap: 12, alignItems: "center", flexWrap: "wrap" }}>
          <Tag color="blue">{user.displayName || user.username}</Tag>
          <span>{user.tenant}{user.school && user.school !== user.tenant ? ` · ${user.school}` : ""}</span>
        </div>
        {auditor ? <div className="identity-start"><h2>按考试、人员和时间查找操作记录</h2><p>查看评分变更、发布和管理操作的详情；导出范围取决于账号权限。</p><Button type="primary" onClick={() => onNavigate("/audit")}>查询操作记录</Button></div> : <div className="identity-start"><h2>查看我的考试与学习反馈</h2><p>使用学校提供的学生端入口查看本人已发布成绩。</p>{import.meta.env.VITE_STUDENT_PORTAL_URL ? <Button type="primary" href={import.meta.env.VITE_STUDENT_PORTAL_URL}>进入学生端</Button> : <p>请从学校发放的学生端网址登录；当前站点尚未配置学生端链接。</p>}</div>}
      </section>
    </div>
  );
}
