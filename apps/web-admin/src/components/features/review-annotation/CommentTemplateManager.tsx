import { useEffect, useState } from "react";
import { Alert, Button, Drawer, Empty, Input, List, Popconfirm, Space, Tag } from "antd";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { getUserErrorMessage } from "../../../api/client";
import type { ReviewCommentTemplate } from "../../../api/reviewAnnotations";

interface TemplateDraft {
  title: string;
  shortcut: string;
  content: string;
}

const emptyDraft: TemplateDraft = { title: "", shortcut: "", content: "" };

export interface CommentTemplateManagerProps {
  open: boolean;
  templates: ReviewCommentTemplate[];
  onClose: () => void;
  onCreate: (input: TemplateDraft) => Promise<unknown>;
  onUpdate: (template: ReviewCommentTemplate, input: TemplateDraft) => Promise<unknown>;
  onDelete: (template: ReviewCommentTemplate) => Promise<unknown>;
}

export function CommentTemplateManager({
  open,
  templates,
  onClose,
  onCreate,
  onUpdate,
  onDelete
}: CommentTemplateManagerProps) {
  const [editing, setEditing] = useState<ReviewCommentTemplate>();
  const [draft, setDraft] = useState<TemplateDraft>(emptyDraft);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string>();

  useEffect(() => {
    if (!open) {
      setEditing(undefined);
      setDraft(emptyDraft);
      setError(undefined);
    }
  }, [open]);

  const beginEdit = (template: ReviewCommentTemplate) => {
    setEditing(template);
    setDraft({ title: template.title, shortcut: template.shortcut, content: template.content });
    setError(undefined);
  };

  const reset = () => {
    setEditing(undefined);
    setDraft(emptyDraft);
    setError(undefined);
  };

  const save = async () => {
    if (!draft.title.trim() || !draft.shortcut.trim() || !draft.content.trim()) {
      setError("名称、快捷码和评语内容均不能为空");
      return;
    }
    setSaving(true);
    setError(undefined);
    try {
      if (editing) await onUpdate(editing, draft);
      else await onCreate(draft);
      reset();
    } catch (saveError) {
      setError(getUserErrorMessage(saveError, "保存常用评语失败"));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Drawer title="个人常用评语" width={480} open={open} onClose={onClose} destroyOnHidden>
      <div className="review-template-form">
        {error ? <Alert type="error" showIcon message={error} /> : null}
        <Input
          aria-label="评语名称"
          placeholder="名称，例如：步骤完整"
          value={draft.title}
          onChange={(event) => setDraft((current) => ({ ...current, title: event.target.value }))}
        />
        <Input
          aria-label="评语快捷码"
          addonBefore="/"
          placeholder="例如 step-ok"
          value={draft.shortcut}
          onChange={(event) => setDraft((current) => ({ ...current, shortcut: event.target.value.toLowerCase() }))}
        />
        <Input.TextArea
          aria-label="评语内容"
          rows={3}
          placeholder="插入后仍可按本份答卷编辑"
          value={draft.content}
          onChange={(event) => setDraft((current) => ({ ...current, content: event.target.value }))}
        />
        <Space>
          <Button type="primary" icon={editing ? <Pencil size={15} /> : <Plus size={15} />} loading={saving} onClick={() => void save()}>
            {editing ? "保存修改" : "新增评语"}
          </Button>
          {editing ? <Button onClick={reset}>取消编辑</Button> : null}
        </Space>
      </div>

      <div className="review-template-list">
        {templates.length === 0 ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有个人常用评语" /> : (
          <List
            dataSource={templates}
            renderItem={(template) => (
              <List.Item
                actions={[
                  <Button key="edit" type="text" icon={<Pencil size={15} />} onClick={() => beginEdit(template)}>编辑</Button>,
                  <Popconfirm
                    key="delete"
                    title="删除这条常用评语？"
                    okText="删除"
                    cancelText="取消"
                    onConfirm={() => onDelete(template)}
                  >
                    <Button type="text" danger icon={<Trash2 size={15} />}>删除</Button>
                  </Popconfirm>
                ]}
              >
                <List.Item.Meta
                  title={<Space><span>{template.title}</span><Tag>/{template.shortcut}</Tag></Space>}
                  description={<><div>{template.content}</div><span className="muted">已使用 {template.usage_count} 次</span></>}
                />
              </List.Item>
            )}
          />
        )}
      </div>
    </Drawer>
  );
}
