import { Component, type ErrorInfo, type ReactNode } from "react";
import { Button, Result } from "antd";
import { Home, RefreshCw } from "lucide-react";

interface ErrorBoundaryProps {
  children: ReactNode;
  fallback?: ReactNode;
  /**
   * Changes to this value clear a captured error. Pass the current route so a
   * navigation recovers the page. Prefer this over a `key` on the boundary:
   * a `key` would remount the whole subtree on every navigation, discarding
   * unsaved editor state and re-issuing the page's data requests even when
   * nothing ever failed.
   */
  resetKey?: string;
}

interface ErrorBoundaryState {
  hasError: boolean;
}

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { hasError: false };

  static getDerivedStateFromError(): ErrorBoundaryState {
    return { hasError: true };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error("页面渲染异常", error, errorInfo.componentStack);
  }

  componentDidUpdate(prevProps: ErrorBoundaryProps) {
    if (this.state.hasError && prevProps.resetKey !== this.props.resetKey) {
      this.setState({ hasError: false });
    }
  }

  private reload = () => {
    window.location.reload();
  };

  // A full reload rather than a hash change plus setState: the failing page may
  // be the dashboard itself, and lazy() caches a rejected chunk forever, so
  // re-rendering in place would just throw again.
  private backToDashboard = () => {
    window.location.hash = "/dashboard";
    window.location.reload();
  };

  render() {
    if (this.state.hasError) {
      if (this.props.fallback !== undefined) {
        return this.props.fallback;
      }
      return (
        <Result
          status="error"
          title="页面出现异常"
          subTitle="页面出现了意外错误，可能是网络不稳定或系统刚完成更新。请重新加载；若多次出现，请联系管理员。"
          extra={[
            <Button key="reload" type="primary" icon={<RefreshCw size={16} />} onClick={this.reload}>
              重新加载
            </Button>,
            <Button key="dashboard" icon={<Home size={16} />} onClick={this.backToDashboard}>
              返回工作台
            </Button>
          ]}
        />
      );
    }
    return this.props.children;
  }
}
