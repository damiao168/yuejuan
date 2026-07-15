import type { ReactNode } from "react";
import { Avatar, Breadcrumb, Button, ConfigProvider, Dropdown, Layout, Menu, Space, theme } from "antd";
import { ArrowLeft, ChevronDown, UserRound } from "lucide-react";
import type { SessionUser } from "../auth/session";
import type { AppRoute } from "../router/routes";
import { hasRouteAccess, routeGroups, visibleRoutes } from "../router/routes";
import { MockBadge } from "./MockBadge";

const { Header, Sider, Content } = Layout;

export function AppLayout({
  user,
  currentRoute,
  children,
  onNavigate,
  onLogout,
  immersive = false
}: {
  user: SessionUser;
  currentRoute: AppRoute;
  children: ReactNode;
  onNavigate: (path: string) => void;
  onLogout: () => void;
  immersive?: boolean;
}) {
  const permittedRoutes = visibleRoutes().filter((route) => hasRouteAccess(user, route));
  const selectedPath = currentRoute.key === "examWorkspace" ? "/exams" : currentRoute.path;
  const menuItems = routeGroups()
    .map((group) => {
      const children = permittedRoutes
        .filter((route) => route.group === group)
        .map((route) => ({
        key: route.path,
        icon: route.icon,
        label: route.title
        }));
      return children.length > 0
        ? {
            key: group,
            label: group,
            type: "group" as const,
            children
          }
        : null;
    })
    .filter((item): item is NonNullable<typeof item> => item !== null);

  return (
    <ConfigProvider
      theme={{
        algorithm: theme.defaultAlgorithm,
        token: {
          colorPrimary: "#1677ff",
          colorSuccess: "#52c41a",
          colorWarning: "#faad14",
          colorError: "#ff4d4f",
          colorInfo: "#13c2c2",
          borderRadius: 6,
          fontFamily: "Inter, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif"
        }
      }}
    >
      <Layout className={immersive ? "app-frame immersive-frame" : "app-frame"}>
        {!immersive ? <Sider width={244} className="sidebar" breakpoint="lg" collapsedWidth={72}>
          <button className="brand-block brand-button" onClick={() => onNavigate("/dashboard")} aria-label="返回工作台">
            <div className="brand-mark">E</div>
            <div className="brand-copy">
              <strong>EduGrade</strong>
              <span>Enterprise</span>
            </div>
          </button>
          <Menu
            mode="inline"
            selectedKeys={[selectedPath]}
            items={menuItems}
            onClick={(item) => onNavigate(item.key)}
            className="side-menu"
          />
        </Sider> : null}
        <Layout>
          <Header className={immersive ? "topbar immersive-topbar" : "topbar"}>
            <div className="topbar-left">
              {immersive ? <Space size="middle">
                <Button type="text" icon={<ArrowLeft size={17} />} aria-label="退出阅卷" onClick={() => onNavigate("/dashboard")}>退出阅卷</Button>
                <strong>{currentRoute.title}</strong>
              </Space> : <Breadcrumb items={[{ title: user.school }, { title: currentRoute.title }]} />}
              {currentRoute.mock ? <MockBadge compact={true} /> : null}
            </div>
            <Space className="topbar-actions">
              <Dropdown
                menu={{
                  items: [
                    { key: "profile", label: user.name },
                    { key: "logout", label: "退出" }
                  ],
                  onClick: ({ key }) => {
                    if (key === "logout") {
                      onLogout();
                    }
                  }
                }}
              >
                <Button className="user-button">
                  <Avatar size={24} icon={<UserRound size={15} />} />
                  <span>{user.name}</span>
                  <ChevronDown size={14} />
                </Button>
              </Dropdown>
            </Space>
          </Header>
          <Content className={immersive ? "workspace immersive-workspace" : "workspace"}>{children}</Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
