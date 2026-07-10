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

export function EmptyState({ title = "暂无数据", description = "当前筛选条件下没有记录。" }: { title?: string; description?: string }) {
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
      message="请求失败"
      description={message}
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
  return <Result status="403" title="无权限" subTitle="当前账号没有访问该页面所需权限。" extra={onBack ? <Button icon={<Home size={16} />} onClick={onBack}>返回首页</Button> : undefined} />;
}

export function NotFoundState({ onBack }: { onBack?: () => void }) {
  return <Result status="404" title="未找到" subTitle="该页面不存在或当前环境未开放。" extra={onBack ? <Button icon={<Home size={16} />} onClick={onBack}>返回首页</Button> : undefined} />;
}
