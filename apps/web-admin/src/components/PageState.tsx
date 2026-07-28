import { Alert, Button, Empty, Result, Spin } from "antd";
import { Home, RefreshCw } from "lucide-react";

export function LoadingState({ label = "加载中" }: { label?: string }) {
  return (
    <div className="state-panel">
      <Spin />
      <span>{label}</span>
    </div>
  );
}

export function EmptyState({ title = "暂无数据", description = "暂时还没有相关记录。" }: { title?: string; description?: string }) {
  return (
    <div className="state-panel">
      <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={<span>{title}</span>} />
      <span className="muted">{description}</span>
    </div>
  );
}

export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <Alert
      type="error"
      showIcon
      message="加载失败"
      description={message || "网络异常，请稍后重试。"}
      action={
        onRetry ? (
          <Button icon={<RefreshCw size={16} />} onClick={onRetry}>
            重试
          </Button>
        ) : undefined
      }
    />
  );
}

export function ForbiddenState({ onBack }: { onBack?: () => void }) {
  return <Result status="403" title="无权限" subTitle="当前账号没有访问该页面的权限。如需使用，请联系学校管理员开通。" extra={onBack ? <Button icon={<Home size={16} />} onClick={onBack}>返回首页</Button> : undefined} />;
}

export function NotFoundState({ onBack }: { onBack?: () => void }) {
  return <Result status="404" title="未找到" subTitle="该页面不存在，或功能尚未开放。" extra={onBack ? <Button icon={<Home size={16} />} onClick={onBack}>返回首页</Button> : undefined} />;
}
