import type { PropsWithChildren, ReactNode } from "react";

export function toneClass(tone?: string): string {
  switch (tone) {
    case "success":
      return "success";
    case "error":
      return "error";
    case "running":
      return "running";
    case "queued":
    case "ready":
      return "queued";
    default:
      return "neutral";
  }
}

export function StatusBadge({ label, tone }: { label: string; tone?: string }) {
  return <span className={`status-badge ${toneClass(tone)}`}>{label}</span>;
}

export function SectionCard({
  title,
  subtitle,
  action,
  children,
  className = ""
}: PropsWithChildren<{ title: string; subtitle?: string; action?: ReactNode; className?: string }>) {
  return (
    <section className={`surface-card ${className}`.trim()}>
      <header className="surface-head">
        <div>
          <h2>{title}</h2>
          {subtitle ? <p>{subtitle}</p> : null}
        </div>
        {action}
      </header>
      {children}
    </section>
  );
}

export function EmptyState({ text }: { text: string }) {
  return <div className="empty-panel">{text}</div>;
}

export function formatMediaKind(item: { isAudioOnly: boolean; extension: string }) {
  return item.isAudioOnly ? "Аудио" : item.extension.toUpperCase().replace(".", "") || "Видео";
}
