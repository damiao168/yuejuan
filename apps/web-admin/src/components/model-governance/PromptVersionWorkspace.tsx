import { useEffect, useMemo, useState } from "react";
import { Alert, Button, Tag } from "antd";
import { FileText, LockKeyhole, RefreshCw } from "lucide-react";
import { ApiClientError } from "../../api/client";
import {
  getCurrentRuntimePrompt,
  type RuntimePrompt,
  type RuntimePromptComponent
} from "../../api/modelGovernance";

const labels: Record<RuntimePromptComponent["key"], string> = {
  base: "评分总规则",
  short_answer: "简答题",
  calculation: "计算题",
  essay: "作文题",
  discussion: "论述题",
  structured: "结构化输出"
};

function promptError(error: unknown) {
  if (error instanceof ApiClientError && error.code === "runtime_prompt_unavailable") {
    return "评分服务当前不可用，无法核对它实际加载的提示词。";
  }
  return error instanceof Error ? error.message : "读取系统提示词失败。";
}

export function PromptVersionWorkspace() {
  const [prompt, setPrompt] = useState<RuntimePrompt>();
  const [selected, setSelected] = useState<RuntimePromptComponent["key"]>("base");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  const load = async () => {
    setLoading(true);
    setError(undefined);
    try {
      const response = await getCurrentRuntimePrompt();
      setPrompt(response.prompt);
      if (!response.prompt.components.some((item) => item.key === selected)) {
        setSelected(response.prompt.components[0]?.key ?? "base");
      }
    } catch (nextError) {
      setError(promptError(nextError));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
    // Runtime content refreshes only on entry or explicit action.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const active = useMemo(
    () => prompt?.components.find((item) => item.key === selected),
    [prompt, selected]
  );

  if (error) {
    return (
      <Alert
        type="warning"
        showIcon
        message="无法读取运行时提示词"
        description={error}
        action={<Button size="small" loading={loading} onClick={() => void load()}>重试</Button>}
      />
    );
  }

  return (
    <div className="prompt-version-workspace" aria-busy={loading}>
      <aside className="prompt-version-index">
        <div className="prompt-version-meta">
          <span>当前运行版本</span>
          <strong>{prompt?.prompt_version ?? "读取中"}</strong>
          <code>{prompt?.bundle_sha256.slice(0, 12) ?? "------------"}</code>
        </div>
        <nav aria-label="系统提示词组成">
          {prompt?.components.map((component) => (
            <button
              type="button"
              key={component.key}
              className={component.key === selected ? "active" : ""}
              onClick={() => setSelected(component.key)}
            >
              <FileText size={15} />
              <span>{labels[component.key]}</span>
              <small>{component.filename}</small>
            </button>
          ))}
        </nav>
      </aside>

      <section className="prompt-version-inspector">
        <header>
          <div>
            <span className="prompt-version-eyebrow">评分服务实际加载内容</span>
            <h2>{active ? labels[active.key] : "系统提示词"}</h2>
            <p>{active?.filename ?? "正在读取运行时清单"}</p>
          </div>
          <div className="prompt-version-actions">
            <Tag icon={<LockKeyhole size={12} />} color="blue">运行时只读</Tag>
            <Button icon={<RefreshCw size={15} />} loading={loading} onClick={() => void load()}>刷新</Button>
          </div>
        </header>

        <pre className="prompt-content-viewer">{active?.content ?? "正在从评分服务读取……"}</pre>

        <footer>
          <span>SHA-256</span>
          <code>{active?.sha256 ?? "-"}</code>
        </footer>
        <Alert
          type="info"
          showIcon
          message="提示词不能在生产页面直接热改"
          description="修改需形成新版本，完成固定题集评测和人工批准后随部署清单启用；当前页面展示的是评分服务真正使用的内容，不是说明文档。"
        />
      </section>
    </div>
  );
}
