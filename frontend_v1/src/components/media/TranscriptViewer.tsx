import { useEffect, useMemo, useRef, useState } from "react";
import { Search } from "lucide-react";
import type { MediaDetailResponse } from "../../models/types";
import { parseTimestamp } from "../../utils/time";
import { useTranslation } from "../../i18n";
import { Tabs } from "../ui/Tabs";
import { EmptyState } from "../ui/EmptyState";
import { TimelineSegment } from "./TimelineSegment";

interface TranscriptViewerProps {
  transcript: MediaDetailResponse["transcript"];
  currentTime: number;
  onSeek: (time: number) => void;
}

export function TranscriptViewer({ transcript, currentTime, onSeek }: TranscriptViewerProps) {
  const { t } = useTranslation();
  const [tab, setTab] = useState("full");
  const [searchQuery, setSearchQuery] = useState("");

  const tabDefs = [
    { key: "full", label: t("transcript.fullText") },
    { key: "segments", label: t("transcript.segments") },
  ];
  const activeRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  const segments = transcript.segments ?? [];
  const paragraphs = transcript.fullTextParagraphs ?? [];

  const query = searchQuery.toLowerCase().trim();

  const matchingIndices = useMemo(() => {
    if (!query) return new Set<number>();
    return new Set(
      segments
        .map((seg, i) => (seg.text.toLowerCase().includes(query) ? i : -1))
        .filter((i) => i >= 0),
    );
  }, [segments, query]);

  const activeIndex = segments.findIndex((seg, i) => {
    const start = parseTimestamp(seg.startLabel);
    const end = parseTimestamp(seg.endLabel);
    if (currentTime >= start && currentTime < end) return true;
    if (i < segments.length - 1) {
      const nextStart = parseTimestamp(segments[i + 1].startLabel);
      if (currentTime >= end && currentTime < nextStart) return true;
    }
    return false;
  });

  useEffect(() => {
    if (tab === "segments" && activeRef.current) {
      activeRef.current.scrollIntoView({ behavior: "smooth", block: "nearest" });
    }
  }, [activeIndex, tab]);

  if (!transcript.hasTranscript) {
    return <EmptyState text={t("transcript.notAvailable")} />;
  }

  // Highlight matching text in a string
  function highlightText(text: string): React.ReactNode {
    if (!query) return text;
    const lower = text.toLowerCase();
    const idx = lower.indexOf(query);
    if (idx < 0) return text;

    const parts: React.ReactNode[] = [];
    let last = 0;
    let pos = idx;
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

  return (
    <div>
      <Tabs tabs={tabDefs} activeKey={tab} onChange={setTab} />

      {/* Search bar */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: "var(--sp-2)",
          padding: "var(--sp-2) var(--sp-3)",
          borderBottom: "1px solid var(--border)",
        }}
      >
        <Search size={13} style={{ color: "var(--text-muted)", flexShrink: 0 }} />
        <input
          type="text"
          placeholder={t("transcript.search")}
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          style={{
            flex: 1,
            background: "none",
            border: "none",
            outline: "none",
            fontSize: "var(--text-sm)",
            color: "var(--text)",
            fontFamily: "inherit",
          }}
        />
        {query && matchingIndices.size > 0 && (
          <span
            style={{
              fontSize: "var(--text-xs)",
              color: "var(--text-muted)",
              flexShrink: 0,
              fontVariantNumeric: "tabular-nums",
            }}
          >
            {matchingIndices.size}
          </span>
        )}
      </div>

      {tab === "full" && (
        <div
          style={{
            maxHeight: 450,
            overflowY: "auto",
            padding: "var(--sp-2) 0",
          }}
        >
          {paragraphs.map((p, i) => (
            <p
              key={i}
              style={{
                color: "var(--text-secondary)",
                lineHeight: "var(--leading-relaxed)",
                fontSize: "var(--text-base)",
                marginTop: i === 0 ? 0 : "var(--sp-3)",
                marginBottom: 0,
              }}
            >
              {highlightText(p)}
            </p>
          ))}
        </div>
      )}

      {tab === "segments" && (
        <div
          ref={scrollContainerRef}
          style={{
            maxHeight: 450,
            overflowY: "auto",
          }}
        >
          {segments.map((seg, i) => (
            <TimelineSegment
              key={seg.index}
              ref={i === activeIndex ? activeRef : undefined}
              segment={seg}
              isActive={i === activeIndex}
              isSearchMatch={matchingIndices.has(i)}
              searchQuery={query}
              onClick={() => onSeek(parseTimestamp(seg.startLabel))}
            />
          ))}
        </div>
      )}
    </div>
  );
}
