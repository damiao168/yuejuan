import type { ReactNode } from "react";

/** Keeps operational table headings, actions and table content aligned. */
export function DataTableShell({
  title,
  description,
  actions,
  children
}: {
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="eg-data-table-shell">
      <div className="eg-data-table-heading">
        <div><h2>{title}</h2>{description ? <p>{description}</p> : null}</div>
        {actions ? <div>{actions}</div> : null}
      </div>
      {children}
    </section>
  );
}
