import { AlertCircle } from "lucide-react";
import type { MediaDetailResponse } from "../../models/types";
import { parseTimestamp } from "../../utils/time";
import { StatusChip } from "../ui/StatusChip";
import { EmptyState } from "../ui/EmptyState";
import { Card } from "../ui/Card";

interface TriggerMatchesProps {
  triggers: MediaDetailResponse["triggers"];
  onSeek: (time: number) => void;
}

export function TriggerMatches({ triggers, onSeek }: TriggerMatchesProps) {
  return (
    <Card
      title="Triggers"
      action={<StatusChip label={triggers.statusLabel} tone={triggers.statusTone} />}
    >
      {(triggers.items ?? []).length === 0 ? (
        <EmptyState
          text={triggers.notice || "No trigger matches found."}
          icon={<AlertCircle size={18} />}
        />
      ) : (
        <div>
          {(triggers.items ?? []).map((item, i) => (
            <div
              key={`${item.ruleName}-${item.timestamp}-${i}`}
              style={{
                padding: "var(--sp-3) 0",
                borderBottom:
                  i < (triggers.items ?? []).length - 1
                    ? "1px solid var(--border)"
                    : "none",
              }}
            >
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: "var(--sp-2)",
                  flexWrap: "wrap",
                  marginBottom: "var(--sp-2)",
                }}
              >
                <span style={{ fontWeight: 600, color: "var(--text)", fontSize: "var(--text-base)" }}>
                  {item.ruleName}
                </span>
                <StatusChip label={item.category} tone={triggers.statusTone} />
              </div>

              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: "var(--sp-3)",
                  marginBottom: "var(--sp-2)",
                }}
              >
                <button
                  onClick={() => onSeek(parseTimestamp(item.timestamp))}
                  style={{
                    background: "none",
                    border: "none",
                    color: "var(--accent)",
                    cursor: "pointer",
                    fontFamily: "monospace",
                    fontSize: "var(--text-sm)",
                    fontWeight: 600,
                    padding: 0,
                    textDecoration: "underline",
                    textUnderlineOffset: 2,
                  }}
                >
                  {item.timestamp}
                </button>
                <span
                  style={{
                    fontWeight: 700,
                    color: "var(--accent)",
                    fontSize: "var(--text-sm)",
                  }}
                >
                  {item.matchedPhrase}
                </span>
              </div>

              <p
                style={{
                  color: "var(--text-muted)",
                  fontSize: "var(--text-sm)",
                  lineHeight: "var(--leading-normal)",
                  margin: 0,
                  display: "-webkit-box",
                  WebkitLineClamp: 2,
                  WebkitBoxOrient: "vertical",
                  overflow: "hidden",
                }}
              >
                {item.segmentText}
              </p>

              {item.hasScreenshot && item.screenshotURL && (
                <a
                  href={item.screenshotURL}
                  target="_blank"
                  rel="noopener noreferrer"
                  style={{ display: "inline-block", marginTop: "var(--sp-2)" }}
                >
                  <img
                    src={item.screenshotURL}
                    alt={`Screenshot: ${item.ruleName}`}
                    style={{
                      maxWidth: 200,
                      borderRadius: "var(--radius-sm)",
                      border: "1px solid var(--border)",
                      display: "block",
                    }}
                  />
                </a>
              )}
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}
