import { useEffect, useState } from "react";
import { BarChart3, Radio, Calendar, Hash } from "lucide-react";
import { api } from "../../api/client";
import type { AnalyticsResponse } from "../../models/types";

const cardStyle: React.CSSProperties = {
  background: "var(--bg-card)",
  border: "1px solid var(--border)",
  borderRadius: "var(--radius-md)",
  padding: "var(--sp-5)",
};

const sectionHeading: React.CSSProperties = {
  display: "flex",
  alignItems: "center",
  gap: "var(--sp-2)",
  fontSize: "var(--text-sm)",
  fontWeight: 700,
  color: "var(--text)",
  marginBottom: "var(--sp-3)",
  textTransform: "uppercase",
  letterSpacing: "var(--tracking-wide)",
};

function formatHours(sec: number) {
  if (!sec || sec <= 0) return "0 ч";
  const h = sec / 3600;
  return h >= 1 ? `${h.toFixed(1)} ч` : `${Math.round(sec / 60)} мин`;
}

export function AnalyticsPage() {
  const [data, setData] = useState<AnalyticsResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .analytics()
      .then(setData)
      .catch(() => setError("Не удалось загрузить аналитику"));
  }, []);

  if (error) {
    return (
      <div style={{ color: "var(--error)", fontSize: "var(--text-sm)" }}>
        {error}
      </div>
    );
  }

  if (!data) {
    return (
      <div style={{ color: "var(--text-muted)", fontSize: "var(--text-sm)" }}>
        Загрузка…
      </div>
    );
  }

  const maxWord = data.topWords[0]?.count ?? 1;
  const maxActivity = data.activity.reduce((m, a) => Math.max(m, a.mediaCount), 1);

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--sp-5)" }}>
      <header
        style={{
          display: "flex",
          alignItems: "center",
          gap: "var(--sp-2)",
        }}
      >
        <BarChart3 size={18} style={{ color: "var(--accent)" }} />
        <h1 style={{ fontSize: "var(--text-lg)", fontWeight: 700, color: "var(--text)", margin: 0 }}>
          Аналитика
        </h1>
      </header>

      {/* Overview metrics */}
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fit, minmax(160px, 1fr))",
          gap: "var(--sp-3)",
        }}
      >
        {data.overview.map((m) => (
          <div key={m.label} style={cardStyle}>
            <div
              style={{
                fontSize: "var(--text-xs)",
                color: "var(--text-muted)",
                textTransform: "uppercase",
                letterSpacing: "var(--tracking-wide)",
                marginBottom: 4,
              }}
            >
              {m.label}
            </div>
            <div style={{ fontSize: "var(--text-xl)", fontWeight: 700, color: "var(--text)" }}>
              {m.value}
            </div>
            <div style={{ fontSize: "var(--text-xs)", color: "var(--text-muted)", marginTop: 4 }}>
              {m.help}
            </div>
          </div>
        ))}
      </div>

      {/* Top words */}
      <section style={cardStyle}>
        <div style={sectionHeading}>
          <Hash size={14} /> Топ слов
        </div>
        {data.topWords.length === 0 ? (
          <div style={{ color: "var(--text-muted)", fontSize: "var(--text-sm)" }}>
            Пока нет данных. Загрузите медиа и дождитесь транскрипции.
          </div>
        ) : (
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            {data.topWords.map((w) => (
              <div
                key={w.word}
                style={{
                  display: "grid",
                  gridTemplateColumns: "160px 1fr 56px",
                  alignItems: "center",
                  gap: "var(--sp-3)",
                  fontSize: "var(--text-sm)",
                }}
              >
                <span style={{ color: "var(--text)", fontWeight: 500 }}>{w.word}</span>
                <div
                  style={{
                    height: 8,
                    borderRadius: 4,
                    background: "var(--bg-inset)",
                    overflow: "hidden",
                  }}
                >
                  <div
                    style={{
                      width: `${Math.round((w.count / maxWord) * 100)}%`,
                      height: "100%",
                      background: "var(--accent)",
                    }}
                  />
                </div>
                <span style={{ color: "var(--text-muted)", textAlign: "right", fontVariantNumeric: "tabular-nums" }}>
                  {w.count}
                </span>
              </div>
            ))}
          </div>
        )}
      </section>

      {/* Sources */}
      <section style={cardStyle}>
        <div style={sectionHeading}>
          <Radio size={14} /> Источники
        </div>
        {data.sources.length === 0 ? (
          <div style={{ color: "var(--text-muted)", fontSize: "var(--text-sm)" }}>Нет данных</div>
        ) : (
          <div style={{ overflow: "auto" }}>
            <table style={{ width: "100%", fontSize: "var(--text-sm)", borderCollapse: "collapse" }}>
              <thead>
                <tr style={{ color: "var(--text-muted)", textAlign: "left" }}>
                  <th style={{ padding: "6px 8px", fontWeight: 600 }}>Источник</th>
                  <th style={{ padding: "6px 8px", fontWeight: 600, textAlign: "right" }}>Медиа</th>
                  <th style={{ padding: "6px 8px", fontWeight: 600, textAlign: "right" }}>Транскр.</th>
                  <th style={{ padding: "6px 8px", fontWeight: 600, textAlign: "right" }}>Длительность</th>
                </tr>
              </thead>
              <tbody>
                {data.sources.map((s) => (
                  <tr key={s.source} style={{ borderTop: "1px solid var(--border)" }}>
                    <td style={{ padding: "6px 8px", color: "var(--text)" }}>{s.source}</td>
                    <td style={{ padding: "6px 8px", textAlign: "right", fontVariantNumeric: "tabular-nums" }}>
                      {s.mediaCount}
                    </td>
                    <td style={{ padding: "6px 8px", textAlign: "right", fontVariantNumeric: "tabular-nums" }}>
                      {s.transcriptCount}
                    </td>
                    <td style={{ padding: "6px 8px", textAlign: "right", color: "var(--text-muted)" }}>
                      {formatHours(s.totalDurationSec)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {/* Activity */}
      <section style={cardStyle}>
        <div style={sectionHeading}>
          <Calendar size={14} /> Активность (по дням)
        </div>
        {data.activity.length === 0 ? (
          <div style={{ color: "var(--text-muted)", fontSize: "var(--text-sm)" }}>Нет данных</div>
        ) : (
          <div
            style={{
              display: "flex",
              alignItems: "flex-end",
              gap: 4,
              height: 120,
              paddingBottom: 4,
              borderBottom: "1px solid var(--border)",
            }}
          >
            {data.activity.map((a) => (
              <div
                key={a.date}
                title={`${a.date}: ${a.mediaCount}`}
                style={{
                  flex: 1,
                  minWidth: 8,
                  background: "var(--accent)",
                  height: `${(a.mediaCount / maxActivity) * 100}%`,
                  borderRadius: "3px 3px 0 0",
                  opacity: 0.8,
                }}
              />
            ))}
          </div>
        )}
        {data.activity.length > 0 && (
          <div
            style={{
              display: "flex",
              justifyContent: "space-between",
              marginTop: 6,
              fontSize: "var(--text-xs)",
              color: "var(--text-muted)",
            }}
          >
            <span>{data.activity[0].date}</span>
            <span>{data.activity[data.activity.length - 1].date}</span>
          </div>
        )}
      </section>
    </div>
  );
}
