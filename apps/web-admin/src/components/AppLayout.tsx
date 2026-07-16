import { useState, type ReactNode } from "react";
import { Avatar, Breadcrumb, Button, ConfigProvider, Drawer, Dropdown, Grid, Layout, Menu, Segmented, Space, theme } from "antd";
import { ArrowLeft, ChevronDown, Menu as MenuIcon, UserRound } from "lucide-react";
import type { SessionUser } from "../auth/session";
import type { AppRoute } from "../router/routes";
import { hasRouteAccess, routeGroups, routePresentation, visibleRoutes } from "../router/routes";
import { experienceLabel, type ProductExperience } from "../router/experience";
import { MockBadge } from "./MockBadge";

const { Header, Sider, Content } = Layout;

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
  const navigation = (
    <>
      <button className="brand-block brand-button" onClick={() => { setNavigationOpen(false); onNavigate("/dashboard"); }} aria-label={`返回${experienceLabel(experience)}工作台`}>
        <div className="brand-mark">E</div>
        <div className="brand-copy">
          <strong>EduGrade</strong>
          <span>{experienceLabel(experience)}</span>
        </div>
      </button>
      {availableExperiences.length > 1 ? (
        <div className="experience-switcher">
          <Segmented
            block
            size="small"
            aria-label="切换产品端"
            value={experience}
            options={availableExperiences.map((value) => ({ value, label: experienceLabel(value) }))}
            onChange={(value) => { setNavigationOpen(false); onExperienceChange(value as ProductExperience); }}
          />
        </div>
      ) : null}
      <Menu
        mode="inline"
        selectedKeys={[selectedPath]}
        items={menuItems}
        onClick={(item) => { setNavigationOpen(false); onNavigate(item.key); }}
        className="side-menu"
      />
    </>
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
        {!immersive && desktopNavigation ? <Sider width={244} className="sidebar">{navigation}</Sider> : null}
        {!immersive && !desktopNavigation ? (
          <Drawer
            className="mobile-navigation"
            title={null}
            placement="left"
            width={300}
            open={navigationOpen}
            onClose={() => setNavigationOpen(false)}
            styles={{ body: { padding: 0 } }}
          >
            {navigation}
          </Drawer>
        ) : null}
        <Layout>
          <Header className={immersive ? "topbar immersive-topbar" : "topbar"}>
            <div className={!immersive && !desktopNavigation ? "topbar-left mobile" : "topbar-left"}>
              {!immersive && !desktopNavigation ? <Button className="mobile-nav-button" type="text" icon={<MenuIcon size={20} />} aria-label="打开主导航" onClick={() => setNavigationOpen(true)} /> : null}
              {immersive ? <Space size="middle">
                <Button type="text" icon={<ArrowLeft size={17} />} aria-label="退出阅卷" onClick={() => onNavigate("/dashboard")}>退出阅卷</Button>
                <strong>{experienceLabel(experience)} · {currentPresentation.title}</strong>
              </Space> : <Breadcrumb items={[{ title: experienceLabel(experience) }, { title: user.school }, { title: currentPresentation.title }]} />}
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
