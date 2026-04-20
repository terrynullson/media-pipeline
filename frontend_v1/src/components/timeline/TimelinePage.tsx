import { useCallback, useEffect, useState } from "react";
import { Clock, Download, Filter } from "lucide-react";
import { api } from "../../api/client";
import type { TimelineItem, TimelineFilters } from "../../models/types";
import { Button } from "../ui/Button";

const inputStyle: React.CSSProperties = {
  padding: "7px 10px",
  borderRadius: "var(--radius-sm)",
  border: "1px solid var(--border-strong)",
  background: "var(--bg-inset)",
  color: "var(--text)",
  fontSize: "var(--text-sm)",
  outline: "none",
};

function formatDate(iso: string) {
  if (!iso || iso.length < 19) return iso;
  return `${iso.slice(0, 10)} ${iso.slice(11, 19)}`;
}

export function TimelinePage() {
  const [filters, setFilters] = useState<TimelineFilters>({});
  const [items, setItems] = useState<TimelineItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async (f: TimelineFilters) => {
    setLoading(true);
    setError(null);
    try {
      const res = await api.timeline(f);
      setItems(res.items);
    } catch {
      setError("Не удалось загрузить хронологию");
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    load({});
  }, [load]);

  function apply() {
    load(filters);
  }

  function reset() {
    setFilters({});
    load({});
  }

  function exportCSV() {
    const url = api.timelineExportURL(filters);
    window.location.href = url;
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--sp-5)" }}>
      <header style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)" }}>
        <Clock size={18} style={{ color: "var(--accent)" }} />
        <h1 style={{ fontSize: "var(--text-lg)", fontWeight: 700, color: "var(--text)", margin: 0 }}>
          Хронология
        </h1>
        <span style={{ color: "var(--text-muted)", fontSize: "var(--text-sm)" }}>
          · посегментная история транскрипций
        </span>
      </header>

      {/* Filters */}
      <section
        style={{
          background: "var(--bg-card)",
          border: "1px solid var(--border)",
          borderRadius: "var(--radius-md)",
          padding: "var(--sp-4)",
          display: "flex",
          flexWrap: "wrap",
          alignItems: "flex-end",
          gap: "var(--sp-3)",
        }}
      >
        <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <label style={{ fontSize: "var(--text-xs)", color: "var(--text-muted)" }}>С (дата/время)</label>
          <input
            type="datetime-local"
            style={inputStyle}
            value={filters.from ?? ""}
            onChange={(e) => setFilters({ ...filters, from: e.target.value })}
          />
        </div>
        <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <label style={{ fontSize: "var(--text-xs)", color: "var(--text-muted)" }}>По</label>
          <input
            type="datetime-local"
            style={inputStyle}
            value={filters.to ?? ""}
            onChange={(e) => setFilters({ ...filters, to: e.target.value })}
          />
        </div>
        <div style={{ display: "flex", flexDirection: "column", gap: 4, minWidth: 160 }}>
          <label style={{ fontSize: "var(--text-xs)", color: "var(--text-muted)" }}>Источник</label>
          <input
            placeholder="Например, Recorder_1"
            style={inputStyle}
            value={filters.source ?? ""}
            onChange={(e) => setFilters({ ...filters, source: e.target.value })}
          />
        </div>

        <div style={{ display: "flex", gap: "var(--sp-2)" }}>
          <Button variant="primary" size="sm" onClick={apply}>
            <Filter size={13} style={{ marginRight: 4 }} /> Применить
          </Button>
          <Button variant="secondary" size="sm" onClick={reset}>
            Сбросить
          </Button>
          <Button variant="secondary" size="sm" onClick={exportCSV}>
            <Download size={13} style={{ marginRight: 4 }} /> Экспорт CSV
          </Button>
        </div>
      </section>

      {error && (
        <div style={{ color: "var(--error)", fontSize: "var(--text-sm)" }}>{error}</div>
      )}

      {/* Items */}
      <section
        style={{
          background: "var(--bg-card)",
          border: "1px solid var(--border)",
          borderRadius: "var(--radius-md)",
          overflow: "hidden",
        }}
      >
        {loading && (
          <div style={{ padding: "var(--sp-4)", color: "var(--text-muted)", fontSize: "var(--text-sm)" }}>
            Загрузка…
          </div>
        )}
        {!loading && items.length === 0 && (
          <div style={{ padding: "var(--sp-4)", color: "var(--text-muted)", fontSize: "var(--text-sm)" }}>
            Нет сегментов под текущий фильтр. Хронология появляется только для медиа с известным временем
            начала записи (парсится из имени файла).
          </div>
        )}
        {!loading && items.length > 0 && (
          <div style={{ overflow: "auto", maxHeight: "70vh" }}>
            <table style={{ width: "100%", fontSize: "var(--text-sm)", borderCollapse: "collapse" }}>
              <thead
                style={{
                  position: "sticky",
                  top: 0,
                  background: "var(--bg-card)",
                  zIndex: 1,
                }}
              >
                <tr style={{ color: "var(--text-muted)", textAlign: "left" }}>
                  <th style={{ padding: "8px 10px", fontWeight: 600, whiteSpace: "nowrap" }}>Дата · время</th>
                  <th style={{ padding: "8px 10px", fontWeight: 600 }}>Источник</th>
                  <th style={{ padding: "8px 10px", fontWeight: 600 }}>Текст</th>
                  <th style={{ padding: "8px 10px", fontWeight: 600 }}>Исправленный</th>
                </tr>
              </thead>
              <tbody>
                {items.map((it, idx) => (
                  <tr
                    key={`${it.mediaId}-${idx}-${it.startSec}`}
                    style={{ borderTop: "1px solid var(--border)", verticalAlign: "top" }}
                  >
                    <td style={{ padding: "8px 10px", whiteSpace: "nowrap", color: "var(--text)", fontVariantNumeric: "tabular-nums" }}>
                      {formatDate(it.segmentStart)}
                    </td>
                    <td style={{ padding: "8px 10px", color: "var(--text-muted)", whiteSpace: "nowrap" }}>
                      {it.source}
                    </td>
                    <td style={{ padding: "8px 10px", color: "var(--text)" }}>{it.text}</td>
                    <td style={{ padding: "8px 10px", color: "var(--text-muted)", fontStyle: "italic" }}>
                      {it.correctedText || "—"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
