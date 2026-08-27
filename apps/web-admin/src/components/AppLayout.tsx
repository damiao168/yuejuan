import { useState, type ReactNode } from "react";
import { Avatar, Button, ConfigProvider, Drawer, Dropdown, Grid, Layout, Menu, Segmented, Space, theme, Tooltip } from "antd";
import { ArrowLeft, ChevronDown, Menu as MenuIcon, PanelLeftClose, PanelLeftOpen, UserRound } from "lucide-react";
import { productIdentityLabel, type SessionUser } from "../auth/session";
import type { AppRoute } from "../router/routes";
import { hasRouteAccess, routeGroups, routePresentation, visibleRoutes } from "../router/routes";
import { experienceLabel, type ProductExperience } from "../router/experience";
import { workspaceLabel } from "../workspaces/registry";
import { MockBadge } from "./MockBadge";

const { Header, Sider, Content } = Layout;
const DESKTOP_NAVIGATION_WIDTH = 192;
const DESKTOP_NAVIGATION_COLLAPSED_WIDTH = 64;
const MOBILE_NAVIGATION_WIDTH = 280;
const NAVIGATION_COLLAPSED_STORAGE_KEY = "edugrade.navigation.collapsed";

export function AppLayout({
  user,
  currentRoute,
  experience,
  availableExperiences,
  children,
  onNavigate,
  onExperienceChange,
  onLogout,
  immersive = false
}: {
  user: SessionUser;
  currentRoute: AppRoute;
  experience: ProductExperience;
  availableExperiences: ProductExperience[];
  children: ReactNode;
  onNavigate: (path: string) => void;
  onExperienceChange: (experience: ProductExperience) => void;
  onLogout: () => void;
  immersive?: boolean;
}) {
  const screens = Grid.useBreakpoint();
  const desktopNavigation = Boolean(screens.lg);
  const [navigationOpen, setNavigationOpen] = useState(false);
  const [navigationCollapsed, setNavigationCollapsed] = useState(() => {
    try {
      return window.localStorage.getItem(NAVIGATION_COLLAPSED_STORAGE_KEY) === "true";
    } catch {
      return false;
    }
  });
  const permittedRoutes = visibleRoutes(experience).filter((route) => hasRouteAccess(user, route, experience));
  const selectedPath = currentRoute.key === "examWorkspace" ? "/exams" : currentRoute.path;
  const currentPresentation = routePresentation(currentRoute, experience);
  const menuItems = routeGroups(experience)
    .map((group) => {
      const children = permittedRoutes
        .filter((route) => routePresentation(route, experience).group === group)
        .map((route) => ({
          key: route.path,
          icon: route.icon,
          label: routePresentation(route, experience).title
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
  const accountMenu = {
    items: [
      {
        key: "account",
        type: "group" as const,
        label: user.displayName && user.displayName !== user.username
          ? `${user.displayName}（${user.username}）`
          : user.username
      },
      { key: "sessions", label: "账户安全" },
      { key: "logout", label: "退出登录" }
    ],
    onClick: ({ key }: { key: string }) => {
      setNavigationOpen(false);
      if (key === "logout") {
        onLogout();
      } else if (key === "sessions") {
        onNavigate("/account/sessions");
      }
    }
  };
  const navigation = (
    <div className={desktopNavigation && navigationCollapsed ? "sidebar-inner is-collapsed" : "sidebar-inner"}>
      <button type="button" className="brand-block brand-button" onClick={() => { setNavigationOpen(false); onNavigate("/dashboard"); }} aria-label={`返回${experienceLabel(experience)}工作台`}>
        <div className="brand-mark">E</div>
        <div className="brand-copy">
          <strong>EduGrade</strong>
          <span title={user.school}>{user.school || productIdentityLabel(user)}</span>
        </div>
      </button>
      <div className="sidebar-main">
        {availableExperiences.length > 1 && !(desktopNavigation && navigationCollapsed) ? (
          <div className="experience-switcher">
            <Segmented
              block
              size="small"
              aria-label="切换管理端或教师端"
              value={experience}
              options={availableExperiences.map((value) => ({ value, label: workspaceLabel(user, value) }))}
              onChange={(value) => { setNavigationOpen(false); onExperienceChange(value as ProductExperience); }}
            />
          </div>
        ) : null}
        <Menu
          mode="inline"
          inlineCollapsed={desktopNavigation && navigationCollapsed}
          selectedKeys={[selectedPath]}
          items={menuItems}
          onClick={(item) => { setNavigationOpen(false); onNavigate(item.key); }}
          className="side-menu"
        />
      </div>
      <div className="sidebar-footer">
        <Dropdown menu={accountMenu} placement="topLeft" trigger={["click"]}>
          <button type="button" className="sidebar-account" aria-label={`账户菜单：${user.displayName || user.username}`}>
            <Avatar size={28} icon={<UserRound size={16} />} />
            <span className="sidebar-account-copy">
              <strong>{user.displayName || user.username}</strong>
              <small>{productIdentityLabel(user)}</small>
            </span>
            <ChevronDown size={14} />
          </button>
        </Dropdown>
      </div>
    </div>
  );

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
        {!immersive && desktopNavigation ? (
          <Sider width={DESKTOP_NAVIGATION_WIDTH} collapsedWidth={DESKTOP_NAVIGATION_COLLAPSED_WIDTH} collapsed={navigationCollapsed} trigger={null} className="sidebar">
            {navigation}
            <Tooltip title={navigationCollapsed ? "展开导航" : "收起导航"} placement="right">
              <button
                type="button"
                className="sidebar-collapse-trigger"
                aria-label={navigationCollapsed ? "展开导航" : "收起导航"}
                onClick={() => {
                  const next = !navigationCollapsed;
                  setNavigationCollapsed(next);
                  try { window.localStorage.setItem(NAVIGATION_COLLAPSED_STORAGE_KEY, String(next)); } catch { /* persistence is optional */ }
                }}
              >
                {navigationCollapsed ? <PanelLeftOpen size={14} /> : <PanelLeftClose size={14} />}
              </button>
            </Tooltip>
          </Sider>
        ) : null}
        {!immersive && !desktopNavigation ? (
          <Drawer
            className="mobile-navigation"
            title={null}
            placement="left"
            width={MOBILE_NAVIGATION_WIDTH}
            open={navigationOpen}
            onClose={() => setNavigationOpen(false)}
            styles={{ body: { padding: 0 } }}
          >
            {navigation}
          </Drawer>
        ) : null}
        <Layout>
          {immersive ? (
            <Header className="topbar immersive-topbar">
              <div className="topbar-left">
                <Space size="middle">
                <Button type="text" icon={<ArrowLeft size={17} />} aria-label="退出阅卷" onClick={() => onNavigate("/dashboard")}>退出阅卷</Button>
                <strong>{experienceLabel(experience)} · {currentPresentation.title}</strong>
                </Space>
                {currentRoute.mock ? <MockBadge compact={true} /> : null}
              </div>
              <Space className="topbar-actions">
                <Dropdown menu={accountMenu} placement="bottomRight" trigger={["click"]}>
                <Button className="user-button">
                  <Avatar size={24} icon={<UserRound size={15} />} />
                  <span>{user.username}</span>
                  <ChevronDown size={14} />
                </Button>
                </Dropdown>
              </Space>
            </Header>
          ) : !desktopNavigation ? (
            <Header className="mobile-shellbar">
              <Button className="mobile-nav-button" type="text" icon={<MenuIcon size={20} />} aria-label="打开主导航" onClick={() => setNavigationOpen(true)} />
              {currentRoute.mock ? <MockBadge compact={true} /> : null}
            </Header>
          ) : null}
          <Content className={immersive ? "workspace immersive-workspace" : "workspace"}>
            {!immersive && desktopNavigation && currentRoute.mock ? <div className="workspace-route-indicator"><MockBadge compact={true} /></div> : null}
            {children}
          </Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
