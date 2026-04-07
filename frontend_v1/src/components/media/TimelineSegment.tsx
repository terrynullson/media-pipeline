import { forwardRef } from "react";
import type { TranscriptSegment } from "../../models/types";

interface TimelineSegmentProps {
  segment: TranscriptSegment;
  isActive: boolean;
  onClick: () => void;
}

export const TimelineSegment = forwardRef<HTMLDivElement, TimelineSegmentProps>(
  function TimelineSegment({ segment, isActive, onClick }, ref) {
    return (
      <div
        ref={ref}
        onClick={onClick}
        role="button"
        tabIndex={0}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            onClick();
          }
        }}
        style={{
          display: "grid",
          gridTemplateColumns: "72px 1fr",
          gap: "var(--sp-3)",
          alignItems: "start",
          padding: "var(--sp-2) var(--sp-3)",
          cursor: "pointer",
          borderLeft: isActive ? "2px solid var(--accent)" : "2px solid transparent",
          background: isActive ? "var(--accent-soft)" : "transparent",
          transition:
            "background var(--duration-fast) var(--ease), border-color var(--duration-fast) var(--ease)",
        }}
        onMouseEnter={(e) => {
          if (!isActive) e.currentTarget.style.background = "var(--bg-card-hover)";
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.background = isActive ? "var(--accent-soft)" : "transparent";
        }}
      >
        <div style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 2 }}>
          <span
            style={{
              fontFamily: "monospace",
              fontSize: "var(--text-xs)",
              background: "var(--bg-inset)",
              borderRadius: "var(--radius-pill)",
              padding: "3px 8px",
              color: "var(--text-muted)",
              whiteSpace: "nowrap",
            }}
          >
            {segment.startLabel}
          </span>
          {segment.hasConfidence && segment.confidence && (
            <span
              style={{
                fontSize: "10px",
                color: "var(--success)",
                background: "var(--success-soft)",
                borderRadius: "var(--radius-pill)",
                padding: "1px 6px",
                fontWeight: 600,
              }}
            >
              {segment.confidence}
            </span>
          )}
        </div>

        <p
          style={{
            margin: 0,
            fontSize: "var(--text-base)",
            lineHeight: "var(--leading-relaxed)",
            color: isActive ? "var(--text)" : "var(--text-secondary)",
          }}
        >
          {segment.text}
        </p>
      </div>
    );
  }
);
