import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { Search, ChevronDown, ChevronRight, FileAudio, ExternalLink, Trash2 } from "lucide-react";
import type { MediaListItem } from "../../models/types";
import { api } from "../../api/client";
import { StatusChip } from "../ui/StatusChip";
import { Progress } from "../ui/Progress";
import { EmptyState } from "../ui/EmptyState";
import { Button } from "../ui/Button";

interface MediaListProps {
  items: MediaListItem[];
  onDeleted?: () => void;
}

type FilterTab = "all" | "processing" | "done" | "failed";

function matchTab(item: MediaListItem, tab: FilterTab): boolean {
  switch (tab) {
    case "processing":
      return item.statusTone === "running" || item.statusTone === "queued";
    case "done":
      return item.statusTone === "success";
    case "failed":
      return item.statusTone === "error";
    default:
      return true;
  }
}

function countTab(items: MediaListItem[], tab: FilterTab): number {
  if (tab === "all") return items.length;
  return items.filter((i) => matchTab(i, tab)).length;
}

const tabs: { key: FilterTab; label: string }[] = [
  { key: "all", label: "All" },
  { key: "processing", label: "Processing" },
  { key: "done", label: "Done" },
  { key: "failed", label: "Failed" },
];

const tabBarStyle: React.CSSProperties = {
  display: "flex",
  alignItems: "center",
  gap: 0,
  borderBottom: "1px solid var(--border)",
  marginBottom: 0,
};

const tabBase: React.CSSProperties = {
  padding: "8px 14px",
  fontSize: "var(--text-sm)",
  fontWeight: 500,
  background: "none",
  border: "none",
  borderBottom: "2px solid transparent",
  cursor: "pointer",
  color: "var(--text-muted)",
  transition: "color var(--duration-fast) var(--ease), border-color var(--duration-fast) var(--ease)",
};

const tabActive: React.CSSProperties = {
  color: "var(--accent)",
  borderBottomColor: "var(--accent)",
  fontWeight: 600,
};

const searchWrap: React.CSSProperties = {
  display: "flex",
  alignItems: "center",
  gap: "var(--sp-2)",
  padding: "8px 14px",
  borderBottom: "1px solid var(--border)",
};

const searchInput: React.CSSProperties = {
  flex: 1,
  background: "none",
  border: "none",
  outline: "none",
  fontSize: "var(--text-sm)",
  color: "var(--text)",
  fontFamily: "inherit",
};

const rowStyle: React.CSSProperties = {
  display: "flex",
  alignItems: "center",
  gap: "var(--sp-3)",
  padding: "10px 14px",
  borderBottom: "1px solid var(--border)",
  color: "inherit",
  transition: "background var(--duration-fast) var(--ease)",
  cursor: "pointer",
};

function stepToneColor(tone: string): string {
  switch (tone) {
    case "success":
      return "var(--success)";
    case "error":
      return "var(--error)";
    case "running":
      return "var(--accent)";
    case "ready":
      return "var(--accent)";
    default:
      return "var(--text-muted)";
  }
}

function isReady(item: MediaListItem): boolean {
  return item.statusTone === "success" || item.statusTone === "error";
}

function MediaRow({ item, onDeleted }: { item: MediaListItem; onDeleted?: () => void }) {
  const [expanded, setExpanded] = useState(false);
  const [showWait, setShowWait] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const steps = item.pipelineSteps ?? [];

  async function handleDelete() {
    setDeleting(true);
    try {
      await api.deleteMedia(item.id);
      onDeleted?.();
    } catch {
      setDeleting(false);
    }
  }

  return (
    <div>
      {/* Main row */}
      <div
        style={rowStyle}
        onClick={() => setExpanded(!expanded)}
        onMouseEnter={(e) => {
          (e.currentTarget as HTMLElement).style.background = "var(--bg-card-hover)";
        }}
        onMouseLeave={(e) => {
          (e.currentTarget as HTMLElement).style.background = "transparent";
        }}
      >
        {/* Expand icon */}
        {expanded ? (
          <ChevronDown size={14} style={{ color: "var(--accent)", flexShrink: 0 }} />
        ) : (
          <ChevronRight size={14} style={{ color: "var(--text-muted)", flexShrink: 0 }} />
        )}

        {/* Name */}
        <span
          style={{
            flex: 1,
            fontWeight: 600,
            fontSize: "var(--text-base)",
            color: "var(--text)",
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
            minWidth: 0,
          }}
        >
          {item.name}
        </span>

        {/* Size */}
        <span
          style={{
            fontSize: "var(--text-sm)",
            color: "var(--text-muted)",
            flexShrink: 0,
            width: 80,
            textAlign: "right",
          }}
        >
          {item.sizeHuman}
        </span>

        {/* Date */}
        <span
          style={{
            fontSize: "var(--text-sm)",
            color: "var(--text-muted)",
            flexShrink: 0,
            width: 140,
            textAlign: "right",
          }}
        >
          {item.createdAtUtc}
        </span>

        {/* Status */}
        <span style={{ flexShrink: 0, width: 100, display: "flex", justifyContent: "center" }}>
          <StatusChip label={item.statusLabel} tone={item.statusTone} />
        </span>
      </div>

      {/* Expanded pipeline steps */}
      {expanded && (
        <div
          style={{
            padding: "var(--sp-2) var(--sp-4) var(--sp-3)",
            paddingLeft: "calc(var(--sp-4) + 14px + var(--sp-3))",
            background: "var(--bg-surface)",
            borderBottom: "1px solid var(--border)",
          }}
        >
          {/* Overall progress bar */}
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: "var(--sp-3)",
              marginBottom: "var(--sp-3)",
            }}
          >
            <span style={{ fontSize: "var(--text-xs)", color: "var(--text-muted)", flexShrink: 0 }}>
              {item.stageLabel}
            </span>
            <div style={{ flex: 1 }}>
              <Progress percent={item.stagePercent} height={4} animate={item.statusTone === "running"} />
            </div>
            <span
              style={{
                fontSize: "var(--text-xs)",
                fontWeight: 600,
                color: "var(--accent)",
                fontVariantNumeric: "tabular-nums",
                minWidth: 32,
                textAlign: "right",
              }}
            >
              {item.stagePercent}%
            </span>
          </div>

          {/* Step list */}
          {steps.length > 0 && (
            <div style={{ display: "flex", flexDirection: "column", gap: "var(--sp-1)" }}>
              {steps.map((step) => (
                <div
                  key={step.label}
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: "var(--sp-3)",
                    padding: "5px 0",
                  }}
                >
                  {/* Step dot */}
                  <span
                    style={{
                      width: 7,
                      height: 7,
                      borderRadius: "50%",
                      background: stepToneColor(step.tone),
                      flexShrink: 0,
                      boxShadow: step.isCurrent ? `0 0 6px ${stepToneColor(step.tone)}` : "none",
                    }}
                  />

                  {/* Step label */}
                  <span
                    style={{
                      fontSize: "var(--text-sm)",
                      fontWeight: step.isCurrent ? 600 : 400,
                      color: step.isCurrent ? "var(--text)" : "var(--text-muted)",
                      minWidth: 160,
                    }}
                  >
                    {step.label}
                  </span>

                  {/* Step status chip */}
                  <StatusChip label={step.statusLabel} tone={step.tone} />

                  {/* Timing */}
                  <span
                    style={{
                      fontSize: "var(--text-xs)",
                      color: "var(--text-muted)",
                      marginLeft: "auto",
                    }}
                  >
                    {step.durationLabel || step.timingText}
                  </span>

                  {/* Step progress bar if running */}
                  {step.progressVisible && step.progressPercent != null && (
                    <div style={{ width: 80 }}>
                      <Progress percent={step.progressPercent} height={3} animate />
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}

          {/* Error message */}
          {item.errorSummary && (
            <div
              style={{
                marginTop: "var(--sp-2)",
                padding: "var(--sp-2) var(--sp-3)",
                background: "rgba(239, 68, 68, 0.08)",
                borderRadius: "var(--radius-sm)",
                border: "1px solid rgba(239, 68, 68, 0.2)",
                fontSize: "var(--text-xs)",
                color: "var(--error)",
              }}
            >
              {item.errorSummary}
            </div>
          )}

          {/* Actions row */}
          <div style={{ marginTop: "var(--sp-3)", display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            {/* Delete action */}
            <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)" }}>
              {!confirmDelete ? (
                <Button
                  variant="ghost"
                  size="sm"
                  icon={<Trash2 size={12} />}
                  onClick={() => setConfirmDelete(true)}
                >
                  Delete
                </Button>
              ) : (
                <>
                  <span style={{ fontSize: "var(--text-xs)", color: "var(--error)", fontWeight: 500 }}>
                    Are you sure?
                  </span>
                  <Button variant="danger" size="sm" loading={deleting} onClick={handleDelete}>
                    Yes, delete
                  </Button>
                  <Button variant="ghost" size="sm" onClick={() => setConfirmDelete(false)} disabled={deleting}>
                    Cancel
                  </Button>
                </>
              )}
            </div>

            {/* Open details */}
            <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-3)" }}>
              {showWait && !isReady(item) && (
                <span
                  style={{
                    fontSize: "var(--text-xs)",
                    color: "var(--text-muted)",
                    animation: "fade-in var(--duration-normal) var(--ease)",
                  }}
                >
                  Processing in progress...
                </span>
              )}
              {isReady(item) ? (
                <Link
                  to={`/media/${item.id}`}
                  style={{
                    display: "inline-flex",
                    alignItems: "center",
                    gap: "var(--sp-1)",
                    fontSize: "var(--text-sm)",
                    fontWeight: 500,
                    color: "var(--accent)",
                    textDecoration: "none",
                  }}
                >
                  Open details
                  <ExternalLink size={12} />
                </Link>
              ) : (
                <button
                  type="button"
                  onClick={() => setShowWait(true)}
                  style={{
                    display: "inline-flex",
                    alignItems: "center",
                    gap: "var(--sp-1)",
                    fontSize: "var(--text-sm)",
                    fontWeight: 500,
                    color: "var(--text-muted)",
                    background: "none",
                    border: "none",
                    cursor: "pointer",
                    padding: 0,
                  }}
                >
                  Open details
                  <ExternalLink size={12} />
                </button>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export function MediaList({ items, onDeleted }: MediaListProps) {
  const [activeTab, setActiveTab] = useState<FilterTab>("all");
  const [query, setQuery] = useState("");

  const sorted = useMemo(
    () =>
      [...items].sort(
        (a, b) => new Date(b.createdAtUtc).getTime() - new Date(a.createdAtUtc).getTime(),
      ),
    [items],
  );

  const filtered = useMemo(() => {
    const q = query.toLowerCase().trim();
    return sorted.filter((item) => {
      if (!matchTab(item, activeTab)) return false;
      if (q && !item.name.toLowerCase().includes(q)) return false;
      return true;
    });
  }, [sorted, activeTab, query]);

  return (
    <div
      style={{
        background: "var(--bg-card)",
        border: "1px solid var(--border)",
        borderRadius: "var(--radius-lg)",
        overflow: "hidden",
      }}
    >
      {/* Tab bar */}
      <div style={tabBarStyle}>
        {tabs.map((tab) => (
          <button
            key={tab.key}
            type="button"
            style={{
              ...tabBase,
              ...(activeTab === tab.key ? tabActive : {}),
            }}
            onClick={() => setActiveTab(tab.key)}
          >
            {tab.label} ({countTab(items, tab.key)})
          </button>
        ))}
      </div>

      {/* Search */}
      <div style={searchWrap}>
        <Search size={14} style={{ color: "var(--text-muted)", flexShrink: 0 }} />
        <input
          type="text"
          placeholder="Filter by filename..."
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          style={searchInput}
        />
      </div>

      {/* Rows */}
      {filtered.length === 0 ? (
        <div style={{ padding: "var(--sp-4)" }}>
          <EmptyState text="No items match the current filter." icon={<FileAudio size={20} />} />
        </div>
      ) : (
        <div>
          {filtered.map((item) => (
            <MediaRow key={item.id} item={item} onDeleted={onDeleted} />
          ))}
        </div>
      )}
    </div>
  );
}
