import { useMemo, useState, useEffect } from "react";
import { BarChart3 } from "lucide-react";
import { api } from "../../api/client";
import { useFetch } from "../../hooks/useFetch";
import type {
  AnalyticsResponse,
  AnalyticsDayActivity,
  AnalyticsSource,
  AnalyticsTopWord,
} from "../../models/types";
import { Accordion } from "../ui/Accordion";
import { Button } from "../ui/Button";
import { EmptyState } from "../ui/EmptyState";

const cardStyle: React.CSSProperties = {
  background: "var(--bg-card)",
  border: "1px solid var(--border)",
  borderRadius: "var(--radius-lg)",
  padding: "var(--sp-5)",
};

const cardTitle: React.CSSProperties = {
  fontSize: "var(--text-sm)",
  fontWeight: 700,
  color: "var(--text)",
  marginBottom: "var(--sp-4)",
  textTransform: "uppercase",
  letterSpacing: "var(--tracking-wide)",
};

const skeletonStyle: React.CSSProperties = {
  background:
    "linear-gradient(90deg, var(--bg-card) 25%, var(--bg-card-hover) 50%, var(--bg-card) 75%)",
  backgroundSize: "200% 100%",
  animation: "skeleton-shimmer 1.4s infinite linear",
  borderRadius: "var(--radius-md)",
};

function formatHours(sec: number): string {
  if (!sec || sec <= 0) return "0 ч";
  const h = sec / 3600;
  return h >= 1 ? `${h.toFixed(1)} ч` : `${Math.round(sec / 60)} мин`;
}

function KpiCards({ overview }: { overview: AnalyticsResponse["overview"] }) {
  return (
    <div
      style={{
        display: "grid",
        gridTemplateColumns:
          window.innerWidth < 768
            ? "repeat(2, 1fr)"
            : "repeat(auto-fit, minmax(180px, 1fr))",
        gap: "var(--sp-3)",
      }}
    >
      {overview.map((m) => (
        <div key={m.label} style={cardStyle} title={m.help}>
          <div
            style={{
              fontSize: "var(--text-xs)",
              color: "var(--text-muted)",
              textTransform: "uppercase",
              letterSpacing: "var(--tracking-wide)",
              marginBottom: 6,
            }}
          >
            {m.label}
          </div>
          <div
            style={{
              fontSize: "var(--text-2xl)",
              fontWeight: 700,
              color: "var(--text)",
              lineHeight: 1.2,
            }}
          >
            {m.value}
          </div>
          <div
            style={{
              fontSize: "var(--text-xs)",
              color: "var(--text-muted)",
              marginTop: 6,
            }}
          >
            {m.help}
          </div>
        </div>
      ))}
    </div>
  );
}

function ActivityBar({ data }: { data: AnalyticsDayActivity[] }) {
  const max = Math.max(...data.map((d) => d.mediaCount), 1);
  const H = 80;
  const barW = Math.max(6, Math.floor(540 / Math.max(data.length, 1)) - 2);
  const totalW = data.length * (barW + 2);

  return (
    <div style={{ width: "100%", overflow: "visible" }}>
      <svg
        width="100%"
        viewBox={`0 0 ${totalW} ${H + 20}`}
        preserveAspectRatio="none"
        style={{ display: "block", overflow: "visible" }}
      >
        {data.map((d, i) => {
          const h = Math.max(2, (d.mediaCount / max) * H);
          const [y, m, day] = d.date.split("-");
          const active = d.mediaCount > 0;
          return (
            <rect
              key={d.date}
              x={i * (barW + 2)}
              y={H - h}
              width={barW}
              height={h}
              rx={2}
              fill={active ? "var(--accent)" : "var(--border)"}
              opacity={active ? 0.85 : 1}
              style={{ cursor: "default", transition: "opacity 120ms" }}
              onMouseEnter={(e) => {
                (e.target as SVGRectElement).style.opacity = "1";
              }}
              onMouseLeave={(e) => {
                (e.target as SVGRectElement).style.opacity = active ? "0.85" : "1";
              }}
            >
              <title>{`${day}.${m}.${y} — ${d.mediaCount} зап.`}</title>
            </rect>
          );
        })}
      </svg>
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          marginTop: 6,
          fontSize: "var(--text-xs)",
          color: "var(--text-muted)",
        }}
      >
        {data.length > 0 && (
          <>
            <span>{formatShortDate(data[0].date)}</span>
            {data.length > 14 && (
              <span>{formatShortDate(data[Math.floor(data.length / 2)].date)}</span>
            )}
            <span>{formatShortDate(data[data.length - 1].date)}</span>
          </>
        )}
      </div>
    </div>
  );
}

function formatShortDate(iso: string): string {
  const [, m, d] = iso.split("-");
  return `${d}.${m}`;
}

function SourcesTable({ sources }: { sources: AnalyticsSource[] }) {
  if (sources.length === 0) {
    return (
      <div style={{ color: "var(--text-muted)", fontSize: "var(--text-sm)" }}>
        Нет данных
      </div>
    );
  }

  const header: React.CSSProperties = {
    fontSize: "var(--text-xs)",
    textTransform: "uppercase",
    letterSpacing: "var(--tracking-wide)",
    color: "var(--text-muted)",
    fontWeight: 600,
  };

  return (
    <div style={{ display: "flex", flexDirection: "column" }}>
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "1fr 100px 140px 120px",
          gap: "var(--sp-3)",
          padding: "8px 12px",
          borderBottom: "1px solid var(--border)",
        }}
      >
        <span style={header}>Источник</span>
        <span style={{ ...header, textAlign: "right" }}>Записей</span>
        <span style={{ ...header, textAlign: "right" }}>Транскрибировано</span>
        <span style={{ ...header, textAlign: "right" }}>Длительность</span>
      </div>
      {sources.map((s, idx) => (
        <div
          key={s.source}
          style={{
            display: "grid",
            gridTemplateColumns: "1fr 100px 140px 120px",
            gap: "var(--sp-3)",
            padding: "10px 12px",
            fontSize: "var(--text-sm)",
            background: idx % 2 === 0 ? "var(--bg-inset)" : "transparent",
            color: "var(--text)",
          }}
        >
          <span>{s.source}</span>
          <span
            style={{
              textAlign: "right",
              fontVariantNumeric: "tabular-nums",
            }}
          >
            {s.mediaCount}
          </span>
          <span
            style={{
              textAlign: "right",
              fontVariantNumeric: "tabular-nums",
            }}
          >
            {s.transcriptCount}
          </span>
          <span
            style={{
              textAlign: "right",
              color: "var(--text-muted)",
              fontVariantNumeric: "tabular-nums",
            }}
          >
            {formatHours(s.totalDurationSec)}
          </span>
        </div>
      ))}
    </div>
  );
}

function WordCloud({ topWords }: { topWords: AnalyticsTopWord[] }) {
  if (topWords.length === 0) {
    return (
      <div style={{ color: "var(--text-muted)", fontSize: "var(--text-sm)" }}>
        Пока нет данных. Загрузите медиа и дождитесь транскрипции.
      </div>
    );
  }
  const topCount = topWords[0].count || 1;
  return (
    <div style={{ display: "flex", flexWrap: "wrap", gap: 4, alignItems: "baseline" }}>
      {topWords.map(({ word, count }) => {
        const ratio = count / topCount;
        const size = 11 + Math.round(ratio * 10);
        const alpha = 0.4 + ratio * 0.6;
        return (
          <span
            key={word}
            title={`${count} вхождений`}
            style={{
              fontSize: size,
              color: `rgba(255, 197, 112, ${alpha})`,
              padding: "2px 6px",
              cursor: "default",
              lineHeight: 1.8,
              transition: "opacity var(--duration-fast) var(--ease)",
            }}
          >
            {word}
          </span>
        );
      })}
    </div>
  );
}

function StopWordsEditor({ onSaved }: { onSaved: () => void }) {
  const [value, setValue] = useState("");
  const [loading, setLoading] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .stopWords()
      .then((r) => setValue(r.stopWords))
      .catch(() => setError("Не удалось загрузить стоп-слова"));
  }, []);

  async function save() {
    setLoading(true);
    setError(null);
    setSaved(false);
    try {
      await api.updateStopWords(value);
      setSaved(true);
      onSaved();
    } catch {
      setError("Не удалось сохранить");
    }
    setLoading(false);
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--sp-3)" }}>
      <p style={{ fontSize: "var(--text-xs)", color: "var(--text-muted)", margin: 0 }}>
        Одно слово в строке или через запятую. Эти слова исключаются из топа.
      </p>
      <textarea
        value={value}
        onChange={(e) => {
          setValue(e.target.value);
          setSaved(false);
        }}
        rows={6}
        style={{
          background: "var(--bg-inset)",
          border: "1px solid var(--border-strong)",
          borderRadius: "var(--radius-sm)",
          padding: "8px 10px",
          fontSize: "var(--text-sm)",
          color: "var(--text)",
          outline: "none",
          fontFamily: "var(--font-mono)",
          resize: "vertical",
        }}
      />
      <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-3)" }}>
        <Button variant="primary" size="sm" onClick={save} loading={loading}>
          Сохранить
        </Button>
        {saved && (
          <span style={{ fontSize: "var(--text-xs)", color: "var(--success)" }}>
            Сохранено
          </span>
        )}
        {error && (
          <span style={{ fontSize: "var(--text-xs)", color: "var(--error)" }}>
            {error}
          </span>
        )}
      </div>
    </div>
  );
}

function AnalyticsSkeleton() {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--sp-5)" }}>
      <div style={{ ...skeletonStyle, height: 40, width: 240 }} />
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))",
          gap: "var(--sp-3)",
        }}
      >
        {[0, 1, 2, 3].map((i) => (
          <div key={i} style={{ ...skeletonStyle, height: 96 }} />
        ))}
      </div>
      <div style={{ ...skeletonStyle, height: 140 }} />
      <div style={{ ...skeletonStyle, height: 220 }} />
    </div>
  );
}

export function AnalyticsPage() {
  const { data, loading, error, reload } = useFetch<AnalyticsResponse>(
    () => api.analytics(),
    []
  );

  const content = useMemo(() => {
    if (loading && !data) return <AnalyticsSkeleton />;
    if (error && !data)
      return (
        <EmptyState
          icon={<BarChart3 size={18} />}
          text="Не удалось загрузить аналитику"
        />
      );
    if (!data) return null;

    return (
      <div style={{ display: "flex", flexDirection: "column", gap: "var(--sp-5)" }}>
        <KpiCards overview={data.overview} />

        <section style={cardStyle}>
          <div style={cardTitle}>Активность за 30 дней</div>
          {data.activity.length === 0 ? (
            <div style={{ color: "var(--text-muted)", fontSize: "var(--text-sm)" }}>
              Нет данных
            </div>
          ) : (
            <ActivityBar data={data.activity} />
          )}
        </section>

        <section style={cardStyle}>
          <div style={cardTitle}>Источники</div>
          <SourcesTable sources={data.sources} />
        </section>

        <section style={cardStyle}>
          <div style={cardTitle}>Топ слов</div>
          <WordCloud topWords={data.topWords} />
        </section>

        <Accordion title="Стоп-слова (исключены из топа)">
          <StopWordsEditor onSaved={reload} />
        </Accordion>
      </div>
    );
  }, [data, loading, error, reload]);

  return (
    <div style={{ animation: "fade-in var(--duration-normal) var(--ease)" }}>
      <div style={{ marginBottom: "var(--sp-6)" }}>
        <h1
          style={{
            fontSize: "var(--text-xl)",
            fontWeight: 700,
            color: "var(--text)",
            margin: 0,
          }}
        >
          Аналитика
        </h1>
        <p
          style={{
            fontSize: "var(--text-sm)",
            color: "var(--text-muted)",
            marginTop: "var(--sp-1)",
          }}
        >
          Общая статистика по всему архиву транскрибаций
        </p>
      </div>
      {content}
    </div>
  );
}
