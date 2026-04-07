interface ProgressProps {
  percent: number;
  height?: number;
  animate?: boolean;
}

export function Progress({ percent, height = 5, animate }: ProgressProps) {
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
