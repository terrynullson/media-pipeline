import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { Search, ChevronDown, ChevronRight, FileAudio, ExternalLink, Trash2, CheckSquare, Square } from "lucide-react";
import type { MediaListItem } from "../../models/types";
import { api } from "../../api/client";
import { useTranslation } from "../../i18n";
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

function MediaRow({ item, onDeleted, selectMode, selected, onToggleSelect }: {
  item: MediaListItem;
  onDeleted?: () => void;
  selectMode?: boolean;
  selected?: boolean;
  onToggleSelect?: (id: number) => void;
}) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(false);
  const [showWait, setShowWait] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [hovered, setHovered] = useState(false);
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
      {/* Inline quick-delete confirmation (shown above row) */}
      {confirmDelete && (
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "var(--sp-3)",
            padding: "8px 14px",
            background: "rgba(239,68,68,0.06)",
            borderBottom: "1px solid rgba(239,68,68,0.2)",
            fontSize: "var(--text-sm)",
          }}
        >
          <span style={{ flex: 1, color: "var(--error)", fontWeight: 500 }}>
            {t("action.confirmDelete")} «{item.name}»
          </span>
          <Button variant="danger" size="sm" loading={deleting} onClick={handleDelete}>
            {t("action.yesDelete")}
          </Button>
          <Button variant="ghost" size="sm" onClick={() => setConfirmDelete(false)} disabled={deleting}>
            {t("action.cancel")}
          </Button>
        </div>
      )}

      {/* Main row */}
      <div
        style={{ ...rowStyle, background: (hovered || selected) ? "var(--bg-card-hover)" : "transparent" }}
        onClick={() => selectMode ? onToggleSelect?.(item.id) : setExpanded(!expanded)}
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
      >
        {selectMode ? (
          <span style={{ flexShrink: 0, color: selected ? "var(--accent)" : "var(--text-muted)" }}>
            {selected ? <CheckSquare size={14} /> : <Square size={14} />}
          </span>
        ) : expanded ? (
          <ChevronDown size={14} style={{ color: "var(--accent)", flexShrink: 0 }} />
        ) : (
          <ChevronRight size={14} style={{ color: "var(--text-muted)", flexShrink: 0 }} />
        )}

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

        <span style={{ flexShrink: 0, width: 100, display: "flex", justifyContent: "center" }}>
          <StatusChip label={item.statusLabel} tone={item.statusTone} />
        </span>

        {/* Quick-delete icon — visible on hover */}
        <button
          type="button"
          title={t("action.delete")}
          onClick={(e) => {
            e.stopPropagation();
            setConfirmDelete(true);
          }}
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            width: 26,
            height: 26,
            borderRadius: "var(--radius-sm)",
            border: "none",
            background: "transparent",
            color: hovered ? "var(--error)" : "transparent",
            cursor: "pointer",
            flexShrink: 0,
            transition: "color var(--duration-fast) var(--ease)",
          }}
        >
          <Trash2 size={13} />
        </button>
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
                  <StatusChip label={step.statusLabel} tone={step.tone} />
                  <span
                    style={{
                      fontSize: "var(--text-xs)",
                      color: "var(--text-muted)",
                      marginLeft: "auto",
                    }}
                  >
                    {step.durationLabel || step.timingText}
                  </span>
                  {step.progressVisible && step.progressPercent != null && (
                    <div style={{ width: 80 }}>
                      <Progress percent={step.progressPercent} height={3} animate />
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}

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

          <div style={{ marginTop: "var(--sp-3)", display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)" }}>
              {!confirmDelete ? (
                <Button
                  variant="ghost"
                  size="sm"
                  icon={<Trash2 size={12} />}
                  onClick={() => setConfirmDelete(true)}
                >
                  {t("action.delete")}
                </Button>
              ) : (
                <>
                  <span style={{ fontSize: "var(--text-xs)", color: "var(--error)", fontWeight: 500 }}>
                    {t("action.confirmDelete")}
                  </span>
                  <Button variant="danger" size="sm" loading={deleting} onClick={handleDelete}>
                    {t("action.yesDelete")}
                  </Button>
                  <Button variant="ghost" size="sm" onClick={() => setConfirmDelete(false)} disabled={deleting}>
                    {t("action.cancel")}
                  </Button>
                </>
              )}
            </div>

            <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-3)" }}>
              {showWait && !isReady(item) && (
                <span
                  style={{
                    fontSize: "var(--text-xs)",
                    color: "var(--text-muted)",
                    animation: "fade-in var(--duration-normal) var(--ease)",
                  }}
                >
                  {t("action.processing")}
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
                  {t("action.openDetails")}
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
                  {t("action.openDetails")}
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
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState<FilterTab>("all");
  const [query, setQuery] = useState("");
  const [selectMode, setSelectMode] = useState(false);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [bulkDeleting, setBulkDeleting] = useState(false);
  const [bulkConfirm, setBulkConfirm] = useState(false);

  const tabs: { key: FilterTab; labelKey: "filter.all" | "filter.processing" | "filter.done" | "filter.failed" }[] = [
    { key: "all", labelKey: "filter.all" },
    { key: "processing", labelKey: "filter.processing" },
    { key: "done", labelKey: "filter.done" },
    { key: "failed", labelKey: "filter.failed" },
  ];

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

  const allSelected = filtered.length > 0 && filtered.every((i) => selected.has(i.id));

  function toggleSelectMode() {
    setSelectMode((v) => !v);
    setSelected(new Set());
    setBulkConfirm(false);
  }

  function toggleItem(id: number) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  }

  function toggleAll() {
    if (allSelected) {
      setSelected(new Set());
    } else {
      setSelected(new Set(filtered.map((i) => i.id)));
    }
  }

  async function handleBulkDelete() {
    if (selected.size === 0) return;
    setBulkDeleting(true);
    try {
      await api.bulkDeleteMedia(Array.from(selected));
      setSelected(new Set());
      setSelectMode(false);
      setBulkConfirm(false);
      onDeleted?.();
    } catch {}
    setBulkDeleting(false);
  }

  return (
    <div
      style={{
        background: "var(--bg-card)",
        border: "1px solid var(--border)",
        borderRadius: "var(--radius-lg)",
        overflow: "hidden",
      }}
    >
      {/* Tab bar + select toggle */}
      <div style={{ ...tabBarStyle, justifyContent: "space-between" }}>
        <div style={{ display: "flex" }}>
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
              {t(tab.labelKey)} ({countTab(items, tab.key)})
            </button>
          ))}
        </div>
        <button
          type="button"
          onClick={toggleSelectMode}
          style={{
            ...tabBase,
            marginRight: "var(--sp-2)",
            color: selectMode ? "var(--accent)" : "var(--text-muted)",
            borderBottom: selectMode ? "2px solid var(--accent)" : "2px solid transparent",
          }}
        >
          {selectMode ? t("action.cancelSelect") : t("action.select")}
        </button>
      </div>

      {/* Search + select all */}
      <div style={searchWrap}>
        {selectMode && (
          <button
            type="button"
            onClick={toggleAll}
            style={{ display: "flex", alignItems: "center", background: "none", border: "none", cursor: "pointer", padding: 0, flexShrink: 0, color: allSelected ? "var(--accent)" : "var(--text-muted)" }}
            title={t("action.selectAll")}
          >
            {allSelected ? <CheckSquare size={14} /> : <Square size={14} />}
          </button>
        )}
        <Search size={14} style={{ color: "var(--text-muted)", flexShrink: 0 }} />
        <input
          type="text"
          placeholder={t("filter.placeholder")}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          style={searchInput}
        />
      </div>

      {/* Bulk action panel */}
      {selectMode && selected.size > 0 && (
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "var(--sp-3)",
            padding: "8px 14px",
            background: "rgba(239,68,68,0.06)",
            borderBottom: "1px solid rgba(239,68,68,0.2)",
            fontSize: "var(--text-sm)",
          }}
        >
          {!bulkConfirm ? (
            <>
              <span style={{ flex: 1, color: "var(--text-secondary)" }}>
                {t("action.selectedCount").replace("{n}", String(selected.size))}
              </span>
              <Button variant="danger" size="sm" icon={<Trash2 size={12} />} onClick={() => setBulkConfirm(true)}>
                {t("action.deleteSelected")}
              </Button>
            </>
          ) : (
            <>
              <span style={{ flex: 1, color: "var(--error)", fontWeight: 500 }}>
                {t("action.confirmDeleteN").replace("{n}", String(selected.size))}
              </span>
              <Button variant="danger" size="sm" loading={bulkDeleting} onClick={handleBulkDelete}>
                {t("action.yesDelete")}
              </Button>
              <Button variant="ghost" size="sm" onClick={() => setBulkConfirm(false)} disabled={bulkDeleting}>
                {t("action.cancel")}
              </Button>
            </>
          )}
        </div>
      )}

      {filtered.length === 0 ? (
        <div style={{ padding: "var(--sp-4)" }}>
          <EmptyState text={t("filter.empty")} icon={<FileAudio size={20} />} />
        </div>
      ) : (
        <div>
          {filtered.map((item) => (
            <MediaRow
              key={item.id}
              item={item}
              onDeleted={onDeleted}
              selectMode={selectMode}
              selected={selected.has(item.id)}
              onToggleSelect={toggleItem}
            />
          ))}
        </div>
      )}
    </div>
  );
}
