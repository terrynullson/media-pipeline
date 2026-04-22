import { useCallback, useEffect, useMemo, useState } from "react";
import { Clock, AlertCircle, Download, Search } from "lucide-react";
import { api } from "../../api/client";
import type {
  TimelineItem,
  TimelineFilters,
  AnalyticsResponse,
} from "../../models/types";
import { Button } from "../ui/Button";
import { EmptyState } from "../ui/EmptyState";

const inputStyle: React.CSSProperties = {
  background: "var(--bg-card)",
  border: "1px solid var(--border)",
  borderRadius: "var(--radius-sm)",
  padding: "7px 10px",
  fontSize: "var(--text-sm)",
  color: "var(--text)",
  outline: "none",
  fontFamily: "inherit",
  colorScheme: "dark",
};

const skeletonStyle: React.CSSProperties = {
  background:
    "linear-gradient(90deg, var(--bg-card) 25%, var(--bg-card-hover) 50%, var(--bg-card) 75%)",
  backgroundSize: "200% 100%",
  animation: "skeleton-shimmer 1.4s infinite linear",
  borderRadius: "var(--radius-sm)",
};

const GRID_COLS = "100px 150px 140px 1fr";
const MAX_ITEMS = 500;

function pad2(n: number): string {
  return n < 10 ? `0${n}` : `${n}`;
}

function toDateTimeLocalValue(d: Date): string {
  const year = d.getFullYear();
  const month = pad2(d.getMonth() + 1);
  const day = pad2(d.getDate());
  const hours = pad2(d.getHours());
  const minutes = pad2(d.getMinutes());
  return `${year}-${month}-${day}T${hours}:${minutes}`;
}

function defaultFilters(): { from: string; to: string; source: string } {
  const now = new Date();
  const start = new Date(now);
  start.setHours(0, 0, 0, 0);
  return {
    from: toDateTimeLocalValue(start),
    to: toDateTimeLocalValue(now),
    source: "",
  };
}

function formatDateRu(iso: string): string {
  const [y, m, d] = iso.split("-");
  return `${d}.${m}.${y}`;
}

function fmtTime(iso: string): string {
  return iso.slice(11, 19);
}

function toApiParam(localValue: string): string {
  if (!localValue) return "";
  const d = new Date(localValue);
  if (isNaN(d.getTime())) return "";
  return d.toISOString();
}

function SegmentRow({
  item,
  prevItem,
  hideSource,
}: {
  item: TimelineItem;
  prevItem?: TimelineItem;
  hideSource: boolean;
}) {
  const [hovered, setHovered] = useState(false);
  const startDate = item.segmentStart.slice(0, 10);
  const prevDate = prevItem?.segmentStart.slice(0, 10);
  const showDate = startDate !== prevDate;

  return (
    <div
      style={{
        display: "grid",
        gridTemplateColumns: hideSource ? "100px 150px 1fr" : GRID_COLS,
        gap: "var(--sp-3)",
        padding: "10px 16px",
        borderBottom: "1px solid var(--border)",
        background: hovered ? "var(--bg-card-hover)" : "transparent",
        transition: "background var(--duration-fast) var(--ease)",
        cursor: "pointer",
        alignItems: "start",
      }}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      onClick={() => window.open(`/app-v1/media/${item.mediaId}`, "_blank")}
    >
      <span
        style={{
          fontSize: "var(--text-xs)",
          color: "var(--text-muted)",
          fontVariantNumeric: "tabular-nums",
        }}
      >
        {showDate ? formatDateRu(startDate) : ""}
      </span>
      <span
        style={{
          fontSize: "var(--text-xs)",
          color: "var(--text-secondary)",
          fontFamily: "var(--font-mono)",
          fontVariantNumeric: "tabular-nums",
        }}
      >
        {fmtTime(item.segmentStart)}
        {item.segmentEnd ? ` → ${fmtTime(item.segmentEnd)}` : ""}
      </span>
      {!hideSource && (
        <span
          style={{
            fontSize: "var(--text-xs)",
            color: "var(--text-muted)",
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
        >
          {item.source}
        </span>
      )}
      <span
        style={{
          fontSize: "var(--text-base)",
          color: "var(--text)",
          lineHeight: "var(--leading-relaxed)",
        }}
      >
        {item.text}
      </span>
    </div>
  );
}

export function HistoryPage() {
  const [form, setForm] = useState(defaultFilters);
  const [items, setItems] = useState<TimelineItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [hasSearched, setHasSearched] = useState(false);
  const [sources, setSources] = useState<string[]>([]);
  const [isNarrow, setIsNarrow] = useState<boolean>(
    typeof window !== "undefined" ? window.innerWidth < 768 : false
  );

  useEffect(() => {
    const onResize = () => setIsNarrow(window.innerWidth < 768);
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  useEffect(() => {
    api
      .analytics()
      .then((a: AnalyticsResponse) =>
        setSources(a.sources.map((s) => s.source).filter(Boolean))
      )
      .catch(() => setSources([]));
  }, []);

  const currentFilters: TimelineFilters = useMemo(
    () => ({
      from: toApiParam(form.from) || undefined,
      to: toApiParam(form.to) || undefined,
      source: form.source || undefined,
    }),
    [form]
  );

  const load = useCallback(async (f: TimelineFilters) => {
    setLoading(true);
    setError(null);
    setHasSearched(true);
    try {
      const res = await api.timeline(f);
      setItems(res.items);
    } catch {
      setError("Не удалось загрузить данные");
      setItems([]);
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    load(currentFilters);
    // run once with defaults
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function onSearch() {
    load(currentFilters);
  }

  const exportUrl = api.timelineExportURL(currentFilters);

  return (
    <div style={{ animation: "fade-in var(--duration-normal) var(--ease)" }}>
      <div style={{ marginBottom: "var(--sp-5)" }}>
        <h1
          style={{
            fontSize: "var(--text-xl)",
            fontWeight: 700,
            color: "var(--text)",
            margin: 0,
          }}
        >
          История эфира
        </h1>
        <p
          style={{
            fontSize: "var(--text-sm)",
            color: "var(--text-muted)",
            marginTop: "var(--sp-1)",
          }}
        >
          Хронологическая лента всех транскрибированных сегментов
        </p>
      </div>

      <div
        style={{
          display: "flex",
          flexWrap: "wrap",
          gap: "var(--sp-3)",
          alignItems: "flex-end",
          marginBottom: "var(--sp-4)",
        }}
      >
        <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <label style={{ fontSize: "var(--text-xs)", color: "var(--text-muted)" }}>
            С
          </label>
          <input
            type="datetime-local"
            value={form.from}
            onChange={(e) => setForm({ ...form, from: e.target.value })}
            style={inputStyle}
          />
        </div>
        <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <label style={{ fontSize: "var(--text-xs)", color: "var(--text-muted)" }}>
            По
          </label>
          <input
            type="datetime-local"
            value={form.to}
            onChange={(e) => setForm({ ...form, to: e.target.value })}
            style={inputStyle}
          />
        </div>
        <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <label style={{ fontSize: "var(--text-xs)", color: "var(--text-muted)" }}>
            Источник
          </label>
          <select
            value={form.source}
            onChange={(e) => setForm({ ...form, source: e.target.value })}
            style={{ ...inputStyle, minWidth: 160 }}
          >
            <option value="">Все</option>
            {sources.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        </div>
        <div style={{ display: "flex", gap: "var(--sp-2)", alignItems: "center" }}>
          <Button variant="primary" size="sm" onClick={onSearch}>
            <Search size={13} style={{ marginRight: 4 }} />
            Найти
          </Button>
          <a
            href={exportUrl}
            download
            style={{ textDecoration: "none" }}
            aria-label="Экспорт CSV"
          >
            <Button variant="ghost" size="sm">
              <Download size={13} style={{ marginRight: 4 }} />
              Экспорт CSV
            </Button>
          </a>
        </div>
      </div>

      {!loading && !error && items.length > 0 && (
        <div
          style={{
            fontSize: "var(--text-xs)",
            color: "var(--text-muted)",
            marginBottom: "var(--sp-3)",
          }}
        >
          Найдено {items.length} сегментов
          {items.length === MAX_ITEMS && " (показаны первые 500 — уточни диапазон)"}
        </div>
      )}

      {loading && (
        <div
          style={{
            background: "var(--bg-card)",
            border: "1px solid var(--border)",
            borderRadius: "var(--radius-lg)",
            overflow: "hidden",
            padding: "var(--sp-4)",
            display: "flex",
            flexDirection: "column",
            gap: "var(--sp-3)",
          }}
        >
          {[0, 1, 2].map((i) => (
            <div key={i} style={{ ...skeletonStyle, height: 36 }} />
          ))}
        </div>
      )}

      {!loading && error && (
        <EmptyState icon={<AlertCircle size={18} />} text={error} />
      )}

      {!loading && !error && hasSearched && items.length === 0 && (
        <EmptyState
          icon={<Clock size={18} />}
          text="За выбранный период ничего не найдено"
        />
      )}

      {!loading && !error && items.length > 0 && (
        <div
          style={{
            background: "var(--bg-card)",
            border: "1px solid var(--border)",
            borderRadius: "var(--radius-lg)",
            overflow: "hidden",
          }}
        >
          <div
            style={{
              display: "grid",
              gridTemplateColumns: isNarrow ? "100px 150px 1fr" : GRID_COLS,
              gap: "var(--sp-3)",
              padding: "8px 16px",
              background: "var(--bg-surface)",
              borderBottom: "1px solid var(--border)",
              position: "sticky",
              top: 48,
              zIndex: 1,
              fontSize: "var(--text-xs)",
              fontWeight: 600,
              color: "var(--text-muted)",
              letterSpacing: "var(--tracking-wide)",
              textTransform: "uppercase",
            }}
          >
            <span>Дата</span>
            <span>Время</span>
            {!isNarrow && <span>Источник</span>}
            <span>Текст</span>
          </div>
          {items.map((it, idx) => (
            <SegmentRow
              key={`${it.mediaId}-${idx}-${it.startSec}`}
              item={it}
              prevItem={idx > 0 ? items[idx - 1] : undefined}
              hideSource={isNarrow}
            />
          ))}
        </div>
      )}
    </div>
  );
}
