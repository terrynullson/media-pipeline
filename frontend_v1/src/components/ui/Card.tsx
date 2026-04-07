import type { ReactNode, CSSProperties } from "react";

interface CardProps {
  title?: string;
  action?: ReactNode;
  children: ReactNode;
  style?: CSSProperties;
  className?: string;
  noPad?: boolean;
}

export function Card({ title, action, children, style, className, noPad }: CardProps) {
  return (
    <div
      className={className}
      style={{
        background: "var(--bg-card)",
        border: "1px solid var(--border)",
        borderRadius: "var(--radius-lg)",
        padding: noPad ? 0 : "var(--sp-4)",
        ...style,
      }}
    >
      {title && (
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: "var(--sp-3)",
            marginBottom: "var(--sp-3)",
            padding: noPad ? "var(--sp-4) var(--sp-4) 0" : undefined,
          }}
        >
          <h3
            style={{
              fontSize: "var(--text-sm)",
              fontWeight: 600,
              color: "var(--text-secondary)",
              letterSpacing: "var(--tracking-wide)",
              textTransform: "uppercase",
            }}
          >
            {title}
          </h3>
          {action}
        </div>
      )}
      {children}
    </div>
  );
}
