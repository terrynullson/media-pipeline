import type { ReactNode } from "react";

interface EmptyStateProps {
  text: string;
  icon?: ReactNode;
}

export function EmptyState({ text, icon }: EmptyStateProps) {
  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        gap: "var(--sp-2)",
        padding: "var(--sp-6) var(--sp-4)",
        color: "var(--text-muted)",
        fontSize: "var(--text-sm)",
        textAlign: "center",
        border: "1px dashed var(--border-strong)",
        borderRadius: "var(--radius-md)",
      }}
    >
      {icon}
      <span>{text}</span>
    </div>
  );
}
