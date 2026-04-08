import { forwardRef } from "react";
import type { TranscriptSegment } from "../../models/types";

interface TimelineSegmentProps {
  segment: TranscriptSegment;
  isActive: boolean;
  isSearchMatch?: boolean;
  searchQuery?: string;
  onClick: () => void;
}

function highlightSearch(text: string, query: string): React.ReactNode {
  if (!query) return text;
  const lower = text.toLowerCase();
  const parts: React.ReactNode[] = [];
  let last = 0;
  let pos = lower.indexOf(query);
  while (pos >= 0) {
    if (pos > last) parts.push(text.slice(last, pos));
    parts.push(
      <mark
        key={pos}
        style={{
          background: "var(--accent-soft)",
          color: "var(--accent)",
          borderRadius: 2,
          padding: "0 1px",
        }}
      >
        {text.slice(pos, pos + query.length)}
      </mark>,
    );
    last = pos + query.length;
    pos = lower.indexOf(query, last);
  }
  if (last < text.length) parts.push(text.slice(last));
  return <>{parts}</>;
}

export const TimelineSegment = forwardRef<HTMLDivElement, TimelineSegmentProps>(
  function TimelineSegment({ segment, isActive, isSearchMatch, searchQuery, onClick }, ref) {
    const highlighted = isActive || isSearchMatch;
    const borderColor = isActive
      ? "var(--accent)"
      : isSearchMatch
        ? "var(--info)"
        : "transparent";
    const bg = isActive
      ? "var(--accent-soft)"
      : isSearchMatch
        ? "var(--info-soft)"
        : "transparent";

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
          borderLeft: `2px solid ${borderColor}`,
          background: bg,
          transition:
            "background var(--duration-fast) var(--ease), border-color var(--duration-fast) var(--ease)",
        }}
        onMouseEnter={(e) => {
          if (!highlighted) e.currentTarget.style.background = "var(--bg-card-hover)";
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.background = highlighted ? bg : "transparent";
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
            color: highlighted ? "var(--text)" : "var(--text-secondary)",
          }}
        >
          {searchQuery ? highlightSearch(segment.text, searchQuery) : segment.text}
        </p>
      </div>
    );
  }
);
