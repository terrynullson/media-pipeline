import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { ChevronDown, Search } from "lucide-react";
import { api } from "../../api/client";
import type { MediaListItem } from "../../models/types";
import { EmptyState, SectionCard, StatusBadge, formatMediaKind } from "../common/ui";

const filters = [
  { key: "all", label: "Все" },
  { key: "queued", label: "В очереди" },
  { key: "running", label: "В обработке" },
  { key: "success", label: "Готово" },
  { key: "error", label: "Ошибка" }
] as const;

export function History() {
  const [items, setItems] = useState<MediaListItem[]>([]);
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<(typeof filters)[number]["key"]>("all");
  const [expanded, setExpanded] = useState<number | null>(null);

  useEffect(() => {
    api.media().then(setItems).catch(() => setItems([]));
  }, []);

  const filtered = useMemo(() => {
    return items.filter((item) => {
      const matchQuery = item.name.toLowerCase().includes(query.toLowerCase());
      const tone = item.statusTone === "queued" ? "queued" : item.statusTone;
      const matchFilter = filter === "all" || tone === filter || item.status === filter;
      return matchQuery && matchFilter;
    });
  }, [filter, items, query]);

  return (
    <div className="page-column">
      <section className="page-hero compact">
        <div>
          <span className="page-eyebrow">Полная история</span>
          <h2>Все media items со статусами, этапами и техническими деталями</h2>
          <p>Фильтр по очереди и раскрытие строки показывают, где именно сейчас находится файл в pipeline.</p>
        </div>
      </section>

      <SectionCard title="История файлов" subtitle="Поиск, фильтры и раскрытие этапов">
        <div className="history-toolbar">
          <label className="toolbar-search">
            <Search size={14} />
            <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Поиск по имени файла" />
          </label>
          <div className="filter-tabs">
            {filters.map((item) => (
              <button key={item.key} type="button" className={`filter-tab${filter === item.key ? " active" : ""}`} onClick={() => setFilter(item.key)}>
                {item.label}
              </button>
            ))}
          </div>
        </div>

        {filtered.length === 0 ? (
          <EmptyState text="Подходящих файлов не найдено." />
        ) : (
          <div className="history-table">
            <div className="history-head">
              <span>Файл</span>
              <span>Тип</span>
              <span>Загружен</span>
              <span>Статус</span>
              <span>Этап</span>
              <span>Действие</span>
            </div>

            {filtered.map((item) => {
              const isExpanded = expanded === item.id;
              return (
                <div key={item.id} className="history-row-wrap">
                  <div className="history-row" onClick={() => setExpanded(isExpanded ? null : item.id)}>
                    <div>
                      <div className="table-item-title">{item.name}</div>
                      <div className="table-item-subtitle">{item.sizeHuman}</div>
                    </div>
                    <div className="table-item-subtitle">{formatMediaKind(item)}</div>
                    <div className="table-item-subtitle">{item.createdAtUtc}</div>
                    <div>
                      <StatusBadge label={item.statusLabel} tone={item.statusTone} />
                    </div>
                    <div>
                      <div className="table-item-title small">{item.currentStage}</div>
                      <div className="table-item-subtitle">{item.currentTimingText || item.stageLabel}</div>
                    </div>
                    <div className="history-actions">
                      <Link to={`/media/${item.id}`} className="inline-link" onClick={(event) => event.stopPropagation()}>
                        Открыть
                      </Link>
                      <ChevronDown size={15} className={isExpanded ? "chevron-open" : ""} />
                    </div>
                  </div>

                  {isExpanded ? (
                    <div className="history-expanded">
                      <div className="history-expanded-grid">
                        <div className="detail-chip">preview: {item.previewReady ? "готов" : "нет"}</div>
                        <div className="detail-chip">transcript: {item.hasTranscript ? "готов" : "нет"}</div>
                        <div className="detail-chip">статус: {item.statusLabel}</div>
                        <div className="detail-chip">ошибка: {item.errorSummary || "нет"}</div>
                      </div>

                      <div className="steps-inline">
                        {item.pipelineSteps.map((step) => (
                          <div key={step.label} className="step-card">
                            <div className="step-title-row">
                              <span>{step.label}</span>
                              <StatusBadge label={step.statusLabel} tone={step.tone} />
                            </div>
                            <div className="step-note">{step.timingText}</div>
                            <div className="step-foot">
                              {step.durationLabel || step.progressLabel || "Нет доп. данных"}
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  ) : null}
                </div>
              );
            })}
          </div>
        )}
      </SectionCard>
    </div>
  );
}
