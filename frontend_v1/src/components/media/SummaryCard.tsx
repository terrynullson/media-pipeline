import { Sparkles } from "lucide-react";
import type { MediaDetailResponse } from "../../models/types";
import { Card } from "../ui/Card";
import { Button } from "../ui/Button";
import { EmptyState } from "../ui/EmptyState";

interface SummaryCardProps {
  summary: MediaDetailResponse["summary"];
  onRequestSummary: () => void;
}

export function SummaryCard({ summary, onRequestSummary }: SummaryCardProps) {
  if (summary.hasSummary) {
    return (
      <Card title="Summary">
        <p
          style={{
            color: "var(--text-secondary)",
            lineHeight: "var(--leading-relaxed)",
            fontSize: "var(--text-base)",
            margin: 0,
          }}
        >
          {summary.text}
        </p>

        {(summary.highlights ?? []).length > 0 && (
          <div
            style={{
              display: "flex",
              flexWrap: "wrap",
              gap: "var(--sp-2)",
              marginTop: "var(--sp-3)",
            }}
          >
            {(summary.highlights ?? []).map((h) => (
              <span
                key={h}
                style={{
                  background: "var(--accent-soft)",
                  color: "var(--accent)",
                  borderRadius: "var(--radius-pill)",
                  fontSize: "var(--text-xs)",
                  padding: "3px 10px",
                  fontWeight: 500,
                  whiteSpace: "nowrap",
                }}
              >
                {h}
              </span>
            ))}
          </div>
        )}

        {(summary.provider || summary.updatedAtUtc) && (
          <p
            style={{
              color: "var(--text-muted)",
              fontSize: "var(--text-xs)",
              marginTop: "var(--sp-2)",
              marginBottom: 0,
            }}
          >
            {summary.provider}
            {summary.provider && summary.updatedAtUtc ? " \u00b7 " : ""}
            {summary.updatedAtUtc}
          </p>
        )}
      </Card>
    );
  }

  if (summary.showAction) {
    return (
      <Card>
        <EmptyState
          text={summary.notice || "No summary available yet."}
          icon={<Sparkles size={20} />}
        />
        <div style={{ display: "flex", justifyContent: "center", marginTop: "var(--sp-3)" }}>
          <Button variant="primary" onClick={onRequestSummary} icon={<Sparkles size={14} />}>
            {summary.actionLabel || "Request Summary"}
          </Button>
        </div>
      </Card>
    );
  }

  if (summary.notice) {
    return (
      <Card>
        <p style={{ color: "var(--text-muted)", fontSize: "var(--text-sm)", margin: 0 }}>
          {summary.notice}
        </p>
      </Card>
    );
  }

  return null;
}
