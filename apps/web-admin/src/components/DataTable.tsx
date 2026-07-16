import { useMemo, useState } from "react";
import { Input, Select, Space, Table, type TableColumnsType, type TableProps } from "antd";
import { Search, SlidersHorizontal } from "lucide-react";
import { EmptyState } from "./PageState";
import { ResponsiveTable } from "./ResponsiveTable";

export interface DataTableProps<T extends object> {
  title: string;
  rows: T[];
  columns: TableColumnsType<T>;
  searchPlaceholder?: string;
  filterLabel?: string;
  filterOptions?: string[];
  rowKey?: TableProps<T>["rowKey"];
}

function getDefaultRowKey<T extends object>(record: T) {
  const maybeIdentified = record as { id?: string | number };
  return maybeIdentified.id != null ? String(maybeIdentified.id) : JSON.stringify(record);
}

export function DataTable<T extends object>({ title, rows, columns, searchPlaceholder, filterLabel, filterOptions = [], rowKey }: DataTableProps<T>) {
  const [keyword, setKeyword] = useState("");
  const [filter, setFilter] = useState<string>();
  const filteredRows = useMemo(() => {
    const normalizedKeyword = keyword.trim().toLocaleLowerCase();
    return rows.filter((row) => {
      const searchable = JSON.stringify(row).toLocaleLowerCase();
      return (!normalizedKeyword || searchable.includes(normalizedKeyword)) && (!filter || searchable.includes(filter.toLocaleLowerCase()));
    });
  }, [filter, keyword, rows]);

  return (
    <section className="workspace-section">
      <div className="section-head">
        <div>
          <h2>{title}</h2>
          <p>{filteredRows.length === rows.length ? `${rows.length} 条记录` : `${filteredRows.length} / ${rows.length} 条记录`}</p>
        </div>
        <Space wrap>
          <Input prefix={<Search size={16} />} placeholder={searchPlaceholder ?? "搜索"} className="toolbar-input" value={keyword} onChange={(event) => setKeyword(event.target.value)} allowClear />
          {filterOptions.length > 0 ? (
            <Select
              className="toolbar-select"
              placeholder={filterLabel ?? "筛选"}
              suffixIcon={<SlidersHorizontal size={16} />}
              options={filterOptions.map((item) => ({ label: item, value: item }))}
              value={filter}
              onChange={setFilter}
              allowClear
            />
          ) : null}
        </Space>
      </div>
      <ResponsiveTable<T>
        rowKey={rowKey ?? getDefaultRowKey}
        dataSource={filteredRows}
        columns={columns}
        pagination={{ pageSize: 5, showSizeChanger: false }}
        locale={{ emptyText: <EmptyState title="暂无记录" description="当前模块没有可显示的数据。" /> }}
        size="middle"
      />
    </section>
  );
}
