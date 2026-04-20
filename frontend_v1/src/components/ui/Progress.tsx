interface ProgressProps {
  percent?: number;
  height?: number;
  animate?: boolean;
  /** Shows a bouncing indeterminate bar (no specific percent needed) */
  indeterminate?: boolean;
}

export function Progress({ percent = 0, height = 5, animate, indeterminate }: ProgressProps) {
  if (indeterminate) {
    return (
      <div
        style={{
          height,
          borderRadius: "var(--radius-pill)",
          background: "var(--bg-inset)",
          overflow: "hidden",
          position: "relative",
        }}
      >
        <div
          style={{
            position: "absolute",
            height: "100%",
            width: "35%",
            borderRadius: "var(--radius-pill)",
            background: "var(--accent)",
            animation: "indeterminate 1.6s cubic-bezier(0.4, 0, 0.6, 1) infinite",
            opacity: 0.7,
          }}
        />
      </div>
    );
  }

  return (
    <div
      style={{
        height,
        borderRadius: "var(--radius-pill)",
        background: "var(--bg-inset)",
        overflow: "hidden",
        position: "relative",
      }}
    >
      <div
        style={{
          height: "100%",
          width: `${Math.min(100, Math.max(0, percent))}%`,
          borderRadius: "var(--radius-pill)",
          background: "var(--accent)",
          transition: "width var(--duration-normal) var(--ease)",
          position: "relative",
          overflow: "hidden",
        }}
      >
        {animate && (
          <div
            style={{
              position: "absolute",
              inset: 0,
              background: "linear-gradient(90deg, transparent, rgba(255,255,255,0.18), transparent)",
              animation: "shimmer 1.8s infinite",
            }}
          />
        )}
      </div>
    </div>
  );
}
