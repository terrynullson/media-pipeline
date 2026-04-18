import { Link } from "react-router-dom";
import { ArrowRight } from "lucide-react";
import type { MediaListItem } from "../../models/types";
import { Progress } from "../ui/Progress";
import { StatusChip } from "../ui/StatusChip";

interface ActiveJobProps {
  item: MediaListItem;
}

const wrapper: React.CSSProperties = {
  display: "flex",
  alignItems: "center",
  gap: "var(--sp-4)",
  padding: "var(--sp-3) var(--sp-4)",
  background: "var(--bg-card)",
  border: "1px solid var(--border)",
  borderLeft: "3px solid var(--accent)",
  borderRadius: "var(--radius-md)",
  textDecoration: "none",
  color: "inherit",
  transition: "background var(--duration-fast) var(--ease)",
  cursor: "pointer",
};

export function ActiveJob({ item }: ActiveJobProps) {
  return (
    <Link
      to={`/media/${item.id}`}
      style={wrapper}
      onMouseEnter={(e) => {
        (e.currentTarget as HTMLElement).style.background = "var(--bg-card-hover)";
      }}
      onMouseLeave={(e) => {
        (e.currentTarget as HTMLElement).style.background = "var(--bg-card)";
      }}
    >
      {/* Left: file info */}
      <div style={{ flex: 1, minWidth: 0 }}>
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "var(--sp-2)",
            marginBottom: 4,
          }}
        >
          <span
            style={{
              fontWeight: 600,
              fontSize: "var(--text-base)",
              color: "var(--text)",
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
            }}
          >
            {item.name}
          </span>
          <StatusChip label={item.statusLabel} tone={item.statusTone} />
        </div>
        <div
          style={{
            fontSize: "var(--text-sm)",
            color: "var(--text-muted)",
            display: "flex",
            alignItems: "center",
            gap: "var(--sp-2)",
          }}
        >
          <span>{item.stageLabel}</span>
          {item.currentTimingText && (
            <>
              <span style={{ opacity: 0.4 }}>&middot;</span>
              <span>{item.currentTimingText}</span>
            </>
          )}
          {item.currentEtaLabel && (
            <>
              <span style={{ opacity: 0.4 }}>&middot;</span>
              <span style={{ color: "var(--accent)" }}>{item.currentEtaLabel}</span>
            </>
          )}
        </div>
      </div>

      {/* Right: progress + arrow */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: "var(--sp-3)",
          flexShrink: 0,
          width: 180,
        }}
      >
        <div style={{ flex: 1 }}>
          <Progress percent={item.stagePercent} height={5} animate />
        </div>
        <span
          style={{
            fontSize: "var(--text-sm)",
            fontWeight: 600,
            color: "var(--accent)",
            fontVariantNumeric: "tabular-nums",
            minWidth: 36,
            textAlign: "right",
          }}
        >
          {item.stagePercent}%
        </span>
        <ArrowRight size={14} style={{ color: "var(--text-muted)", flexShrink: 0 }} />
      </div>
    </Link>
  );
}
