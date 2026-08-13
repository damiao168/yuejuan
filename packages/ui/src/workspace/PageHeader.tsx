import type { ReactNode } from "react";

/**
 * A compact operational page heading. It deliberately leaves buttons and
 * inputs to the host application instead of wrapping the component library's
 * controls.
 */
export function PageHeader({
  title,
  eyebrow,
  description,
  actions,
  className
}: {
  title: ReactNode;
  eyebrow?: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  className?: string;
}) {
  return (
    <header className={`eg-page-header ${className ?? ""}`.trim()}>
      <div className="eg-page-header-copy">
        {eyebrow ? <span>{eyebrow}</span> : null}
        <h1>{title}</h1>
        {description ? <p>{description}</p> : null}
      </div>
      {actions ? <div className="eg-page-header-actions">{actions}</div> : null}
    </header>
  );
}
