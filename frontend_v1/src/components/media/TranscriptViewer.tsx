import { useEffect, useRef, useState } from "react";
import type { MediaDetailResponse } from "../../models/types";
import { parseTimestamp } from "../../utils/time";
import { Tabs } from "../ui/Tabs";
import { EmptyState } from "../ui/EmptyState";
import { TimelineSegment } from "./TimelineSegment";

interface TranscriptViewerProps {
  transcript: MediaDetailResponse["transcript"];
  currentTime: number;
  onSeek: (time: number) => void;
}

const tabDefs = [
  { key: "full", label: "Full Text" },
  { key: "segments", label: "Segments" },
];

export function TranscriptViewer({ transcript, currentTime, onSeek }: TranscriptViewerProps) {
  const [tab, setTab] = useState("full");
  const activeRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  const activeIndex = transcript.segments.findIndex((seg, i) => {
    const start = parseTimestamp(seg.startLabel);
    const end = parseTimestamp(seg.endLabel);
    if (currentTime >= start && currentTime < end) return true;
    // If between segments, attach to the one whose start is closest behind
    if (i < transcript.segments.length - 1) {
      const nextStart = parseTimestamp(transcript.segments[i + 1].startLabel);
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
    return <EmptyState text="Transcript not available yet." />;
  }

  return (
    <div>
      <Tabs tabs={tabDefs} activeKey={tab} onChange={setTab} />

      {tab === "full" && (
        <div
          style={{
            maxHeight: 450,
            overflowY: "auto",
            padding: "var(--sp-2) 0",
          }}
        >
          {transcript.fullTextParagraphs.map((p, i) => (
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
              {p}
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
          {transcript.segments.map((seg, i) => (
            <TimelineSegment
              key={seg.index}
              ref={i === activeIndex ? activeRef : undefined}
              segment={seg}
              isActive={i === activeIndex}
              onClick={() => onSeek(parseTimestamp(seg.startLabel))}
            />
          ))}
        </div>
      )}
    </div>
  );
}
