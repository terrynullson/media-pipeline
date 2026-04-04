import type { PropsWithChildren, ReactNode } from "react";

export function SectionHeader(props: { eyebrow?: string; title: string; description?: string; actions?: ReactNode }) {
  return (
    <div className="section-header">
      <div>
        {props.eyebrow ? <div className="eyebrow">{props.eyebrow}</div> : null}
        <h1 className="page-title">{props.title}</h1>
        {props.description ? <p className="page-description">{props.description}</p> : null}
      </div>
      {props.actions ? <div className="section-actions">{props.actions}</div> : null}
    </div>
  );
}

export function Card(props: PropsWithChildren<{ title?: string; subtitle?: string; aside?: ReactNode; className?: string }>) {
  return (
    <section className={`panel-card${props.className ? ` ${props.className}` : ""}`}>
      {props.title || props.subtitle || props.aside ? (
        <div className="panel-card-header">
          <div>
            {props.title ? <h2 className="panel-title">{props.title}</h2> : null}
            {props.subtitle ? <p className="panel-subtitle">{props.subtitle}</p> : null}
          </div>
          {props.aside}
        </div>
      ) : null}
      {props.children}
    </section>
  );
}

export function StatusBadge(props: { label: string; tone?: string }) {
  return <span className={`status-badge ${props.tone ?? "neutral"}`}>{props.label}</span>;
}

export function EmptyState(props: { text: string }) {
  return <div className="empty-state">{props.text}</div>;
}
