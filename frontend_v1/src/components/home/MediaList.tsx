import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { Search, ArrowRight, FileAudio } from "lucide-react";
import type { MediaListItem } from "../../models/types";
import { StatusChip } from "../ui/StatusChip";
import { EmptyState } from "../ui/EmptyState";

interface MediaListProps {
  items: MediaListItem[];
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
  textDecoration: "none",
  color: "inherit",
  transition: "background var(--duration-fast) var(--ease)",
};

export function MediaList({ items }: MediaListProps) {
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
            <Link
              key={item.id}
              to={`/media/${item.id}`}
              style={rowStyle}
              onMouseEnter={(e) => {
                (e.currentTarget as HTMLElement).style.background = "var(--bg-card-hover)";
              }}
              onMouseLeave={(e) => {
                (e.currentTarget as HTMLElement).style.background = "transparent";
              }}
            >
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

              {/* Arrow */}
              <ArrowRight
                size={14}
                style={{ color: "var(--text-muted)", flexShrink: 0 }}
              />
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
