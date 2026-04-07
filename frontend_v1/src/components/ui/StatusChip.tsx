const toneMap: Record<string, { bg: string; color: string }> = {
  success: { bg: "var(--success-soft)", color: "var(--success)" },
  error: { bg: "var(--error-soft)", color: "var(--error)" },
  running: { bg: "var(--warning-soft)", color: "var(--warning)" },
  queued: { bg: "var(--accent-soft)", color: "var(--accent)" },
  neutral: { bg: "rgba(232,209,197,0.06)", color: "var(--text-muted)" },
};

function resolve(tone: string) {
  return toneMap[tone] ?? toneMap.neutral;
}

interface StatusChipProps {
  label: string;
  tone: string;
}

export function StatusChip({ label, tone }: StatusChipProps) {
  const t = resolve(tone);
  return (
    <span
      style={{
        display: "inline-flex",
        alignItems: "center",
        padding: "3px 9px",
        borderRadius: "var(--radius-pill)",
        fontSize: "var(--text-xs)",
        fontWeight: 600,
        letterSpacing: "0.02em",
        lineHeight: 1.4,
        background: t.bg,
        color: t.color,
        whiteSpace: "nowrap",
      }}
    >
      {label}
    </span>
  );
}
