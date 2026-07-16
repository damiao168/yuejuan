import { Fragment, useMemo } from "react";
import { Empty, Grid, Spin, Table, type TableColumnsType, type TableProps } from "antd";
import type { ColumnGroupType, ColumnType } from "antd/es/table";
import type { Key, ReactNode } from "react";

type ResponsiveTableProps<T extends object> = Omit<TableProps<T>, "columns"> & {
  columns: TableColumnsType<T>;
  mobilePrimaryCount?: number;
};

function flattenColumns<T extends object>(columns: TableColumnsType<T>): ColumnType<T>[] {
  return columns.flatMap((column) => {
    const group = column as ColumnGroupType<T>;
    return group.children ? flattenColumns(group.children) : [column as ColumnType<T>];
  });
}

function flexibleColumns<T extends object>(columns: TableColumnsType<T>): TableColumnsType<T> {
  return columns.map((column) => {
    const group = column as ColumnGroupType<T>;
    if (group.children) return { ...group, children: flexibleColumns(group.children) };
    return { ...column, width: undefined, minWidth: undefined, fixed: undefined, ellipsis: false } as ColumnType<T>;
  });
}

function valueAt(record: object, dataIndex: ColumnType<object>["dataIndex"]): unknown {
  if (dataIndex === undefined) return undefined;
  const path = Array.isArray(dataIndex) ? dataIndex : [dataIndex];
  return path.reduce<unknown>((value, part) => {
    if (!value || typeof value !== "object") return undefined;
    return (value as Record<string, unknown>)[String(part)];
  }, record);
}

function columnLabel<T extends object>(column: ColumnType<T>): ReactNode {
  return typeof column.title === "function" ? "详情" : column.title ?? "详情";
}

function renderedValue<T extends object>(column: ColumnType<T>, record: T, index: number): ReactNode {
  const raw = valueAt(record, column.dataIndex as ColumnType<object>["dataIndex"]);
  const rendered = column.render ? column.render(raw, record, index) : raw;
  if (rendered && typeof rendered === "object" && !Array.isArray(rendered) && "children" in rendered && "props" in rendered) {
    return (rendered as { children?: ReactNode }).children ?? null;
  }
  if (rendered === null || rendered === undefined || rendered === "") return <span className="muted">-</span>;
  if (["string", "number", "boolean"].includes(typeof rendered)) return String(rendered);
  return rendered as ReactNode;
}

function recordKey<T extends object>(rowKey: TableProps<T>["rowKey"], record: T, index: number): Key {
  if (typeof rowKey === "function") return rowKey(record);
  if (typeof rowKey === "string") return String((record as Record<string, unknown>)[rowKey] ?? index);
  return String((record as Record<string, unknown>).key ?? index);
}

export function ResponsiveTable<T extends object>({
  columns,
  dataSource = [],
  rowKey,
  loading,
  locale,
  onRow,
  rowClassName,
  className,
  mobilePrimaryCount = 3,
  scroll: _scroll,
  ...tableProps
}: ResponsiveTableProps<T>) {
  const screens = Grid.useBreakpoint();
  const mobile = !screens.xxl;
  const flatColumns = useMemo(() => flattenColumns(columns), [columns]);
  const desktopColumns = useMemo(() => flexibleColumns(columns), [columns]);

  if (!mobile) {
    return (
      <Table<T>
        {...tableProps}
        columns={desktopColumns}
        dataSource={dataSource}
        rowKey={rowKey}
        loading={loading}
        locale={locale}
        onRow={onRow}
        rowClassName={rowClassName}
        className={`responsive-desktop-table ${className ?? ""}`.trim()}
        tableLayout="fixed"
      />
    );
  }

  if (loading) {
    return <div className="responsive-table-loading"><Spin /></div>;
  }
  if (!dataSource.length) {
    const emptyText = typeof locale?.emptyText === "function" ? locale.emptyText() : locale?.emptyText;
    return <div className="responsive-table-empty">{emptyText ?? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} />}</div>;
  }

  return (
    <div className="responsive-record-list" role="list">
      {dataSource.map((record, index) => {
        const primaryColumns = flatColumns.filter((column, columnIndex) => columnIndex < mobilePrimaryCount || columnLabel(column) === "操作");
        const detailColumns = flatColumns.filter((column) => !primaryColumns.includes(column));
        const rowProps = onRow?.(record, index) ?? {};
        const className = typeof rowClassName === "function" ? rowClassName(record, index, 0) : rowClassName;
        return (
          <article
            {...rowProps}
            className={`responsive-record ${className ?? ""}`.trim()}
            key={recordKey(rowKey, record, index)}
            role="listitem"
          >
            <dl>
              {primaryColumns.map((column, columnIndex) => (
                <Fragment key={String(column.key ?? column.dataIndex ?? columnIndex)}>
                  <div className="responsive-record-field">
                    <dt>{columnLabel(column)}</dt>
                    <dd>{renderedValue(column, record, index)}</dd>
                  </div>
                </Fragment>
              ))}
            </dl>
            {detailColumns.length ? (
              <details className="responsive-record-details">
                <summary>查看完整信息</summary>
                <dl>
                  {detailColumns.map((column, columnIndex) => (
                    <div className="responsive-record-field" key={String(column.key ?? column.dataIndex ?? columnIndex)}>
                      <dt>{columnLabel(column)}</dt>
                      <dd>{renderedValue(column, record, index)}</dd>
                    </div>
                  ))}
                </dl>
              </details>
            ) : null}
          </article>
        );
      })}
    </div>
  );
}
