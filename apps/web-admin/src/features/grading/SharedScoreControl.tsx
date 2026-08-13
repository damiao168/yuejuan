import { Button, Checkbox, InputNumber, Space } from "antd";
import { RubricCriterion } from "@edugrade/ui";
import type { RubricPoint } from "../../api/papers";
import { StatusTag } from "../../components/StatusTag";

export interface SharedScoreValue {
  score: number | null;
  rubricSelections: Record<string, number>;
}

export function SharedScoreControl({
  value,
  maxScore,
  rubricPoints,
  disabled = false,
  onChange
}: {
  value: SharedScoreValue;
  maxScore: number;
  rubricPoints: RubricPoint[];
  disabled?: boolean;
  onChange: (value: SharedScoreValue) => void;
}) {
  const rubricTotal = rubricPoints.reduce((total, point) => total + (value.rubricSelections[point.id] ?? 0), 0);
  const setPoint = (point: RubricPoint, score: number) => onChange({
    ...value,
    rubricSelections: { ...value.rubricSelections, [point.id]: score }
  });

  return (
    <>
      <div className="score-input-row">
        <InputNumber
          aria-label="最终得分"
          disabled={disabled}
          min={0}
          max={maxScore || undefined}
          precision={1}
          value={value.score}
          placeholder="最终分"
          onChange={(score) => onChange({ ...value, score: score === null ? null : Number(score) })}
        />
        <span>/ {maxScore || "-"}</span>
      </div>

      {rubricPoints.length > 0 ? (
        <div className="rubric-score-list">
          <div className="rubric-score-head">
            <strong>评分细则</strong>
            <Space size={8} wrap>
              <span>{rubricTotal} 分</span>
              {value.score !== null && rubricTotal > 0 && value.score !== rubricTotal
                ? <StatusTag tone="warning">与最终分不一致</StatusTag>
                : null}
              <Button size="small" disabled={disabled} onClick={() => onChange({ ...value, score: rubricTotal })}>
                填入最终分
              </Button>
            </Space>
          </div>
          {rubricPoints.map((point) => (
            <RubricCriterion
              key={point.id}
              label={point.description || "未命名采分点"}
              score={point.score}
              required={point.required}
              leading={
                <Checkbox
                  disabled={disabled}
                  checked={(value.rubricSelections[point.id] ?? 0) > 0}
                  onChange={(event) => setPoint(point, event.target.checked ? point.score : 0)}
                />
              }
            >
              <InputNumber
                aria-label={`${point.description || "采分点"}得分`}
                disabled={disabled}
                min={0}
                max={point.score}
                precision={1}
                value={value.rubricSelections[point.id] ?? 0}
                onChange={(score) => setPoint(point, Number(score ?? 0))}
              />
            </RubricCriterion>
          ))}
        </div>
      ) : <p className="grading-inline-note">当前题目没有评分细则，请直接填写最终得分。</p>}
    </>
  );
}
