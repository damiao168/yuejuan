import { Fragment, useEffect, useMemo, useState, type KeyboardEvent } from "react";
import { Empty, Grid, Pagination, Spin, Table, type TableColumnsType, type TableProps } from "antd";
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
  pagination,
  scroll,
  ...tableProps
}: ResponsiveTableProps<T>) {
  const screens = Grid.useBreakpoint();
  const mobile = !screens.lg;
  const flatColumns = useMemo(() => flattenColumns(columns), [columns]);
  const paginationConfig = pagination === false ? null : (pagination ?? {});
  const [mobilePage, setMobilePage] = useState(paginationConfig?.defaultCurrent ?? 1);
  const [mobilePageSize, setMobilePageSize] = useState(paginationConfig?.defaultPageSize ?? 10);
  const pageSize = paginationConfig?.pageSize ?? mobilePageSize;
  const total = paginationConfig?.total ?? dataSource.length;
  const currentPage = paginationConfig?.current ?? mobilePage;
  const controlledPagination = paginationConfig?.current !== undefined;
  const serverPaginated = typeof paginationConfig?.total === "number" && paginationConfig.total > dataSource.length;
  const mobileDataSource = !paginationConfig || serverPaginated
    ? dataSource
    : dataSource.slice((currentPage - 1) * pageSize, currentPage * pageSize);

  useEffect(() => {
    if (!paginationConfig || controlledPagination) return;
    const lastPage = Math.max(1, Math.ceil(total / Math.max(1, pageSize)));
    setMobilePage((current) => Math.min(current, lastPage));
  }, [controlledPagination, pageSize, paginationConfig !== null, total]);

  const accessibleOnRow = onRow
    ? (record: T, index?: number) => {
        const rowProps = onRow(record, index);
        if (!rowProps.onClick) return rowProps;
        const originalKeyDown = rowProps.onKeyDown;
        return {
          ...rowProps,
          tabIndex: rowProps.tabIndex ?? 0,
          onKeyDown: (event: KeyboardEvent<HTMLElement>) => {
            originalKeyDown?.(event);
            if (
              event.target === event.currentTarget
              && !event.defaultPrevented
              && (event.key === "Enter" || event.key === " ")
            ) {
              event.preventDefault();
              event.currentTarget.click();
            }
          }
        };
      }
    : undefined;

  if (!mobile) {
    return (
      <Table<T>
        {...tableProps}
        columns={columns}
        dataSource={dataSource}
        rowKey={rowKey}
        loading={loading}
        locale={locale}
        onRow={accessibleOnRow}
        rowClassName={rowClassName}
        className={`responsive-desktop-table ${className ?? ""}`.trim()}
        tableLayout="fixed"
        pagination={pagination}
        scroll={scroll}
        size={tableProps.size ?? "small"}
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
    <>
      <div className="responsive-record-list" role="list">
      {mobileDataSource.map((record, pageIndex) => {
        const index = serverPaginated ? pageIndex : (currentPage - 1) * pageSize + pageIndex;
        const primaryColumns = flatColumns.filter((column, columnIndex) => columnIndex < mobilePrimaryCount || columnLabel(column) === "操作");
        const detailColumns = flatColumns.filter((column) => !primaryColumns.includes(column));
        const rowProps = accessibleOnRow?.(record, index) ?? {};
        const className = typeof rowClassName === "function" ? rowClassName(record, index, 0) : rowClassName;
        return (
          <article
            {...rowProps}
            className={`responsive-record ${className ?? ""}`.trim()}
            key={recordKey(rowKey, record, index)}
            role={rowProps.onClick ? "button" : "listitem"}
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
      {paginationConfig && total > pageSize ? (
        <Pagination
          className="responsive-record-pagination"
          current={currentPage}
          pageSize={pageSize}
          total={total}
          showSizeChanger={paginationConfig.showSizeChanger}
          showQuickJumper={paginationConfig.showQuickJumper}
          showTotal={paginationConfig.showTotal}
          pageSizeOptions={paginationConfig.pageSizeOptions}
          onChange={(page, nextPageSize) => {
            setMobilePage(page);
            setMobilePageSize(nextPageSize);
            paginationConfig.onChange?.(page, nextPageSize);
          }}
          onShowSizeChange={(page, nextPageSize) => {
            setMobilePage(page);
            setMobilePageSize(nextPageSize);
            paginationConfig.onShowSizeChange?.(page, nextPageSize);
          }}
        />
      ) : null}
    </>
  );
}
