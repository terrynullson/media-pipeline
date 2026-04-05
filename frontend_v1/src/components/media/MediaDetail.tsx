import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { ListChecks, MonitorPlay, ScrollText, Settings2 } from "lucide-react";
import { api } from "../../api/client";
import type { MediaDetailResponse } from "../../models/types";
import { EmptyState, SectionCard, StatusBadge } from "../common/ui";

export function MediaDetail() {
  const { mediaId = "" } = useParams();
  const [data, setData] = useState<MediaDetailResponse | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    api.mediaDetail(mediaId).then(setData).catch(() => setError("Не удалось загрузить детали файла."));
  }, [mediaId]);

  if (error) {
    return <div className="page-column"><EmptyState text={error} /></div>;
  }

  if (!data) {
    return <div className="page-column"><EmptyState text="Загружаем детали media item..." /></div>;
  }

  const current = data;

  async function handleSummaryRequest() {
    try {
      await api.requestSummary(current.summary.requestSummaryUrl);
      const refreshed = await api.mediaDetail(mediaId);
      setData(refreshed);
    } catch {
      setError("Не удалось запросить summary.");
    }
  }

  return (
    <div className="page-column">
      <section className="page-hero compact">
        <div>
          <span className="page-eyebrow">Media item</span>
          <h2>{current.media.name}</h2>
          <p>{current.media.sizeHuman} • {current.media.createdAtUtc} • {current.media.mimeType}</p>
        </div>
        <StatusBadge label={current.pipeline.statusLabel} tone={current.pipeline.statusTone} />
      </section>

      <div className="detail-grid">
        <SectionCard title="Pipeline" subtitle={current.pipeline.stageLabel} action={<ListChecks size={16} className="panel-icon" />}>
          <div className="steps-inline">
            {current.pipeline.steps.map((step) => (
              <div key={step.label} className="step-card">
                <div className="step-title-row">
                  <span>{step.label}</span>
                  <StatusBadge label={step.statusLabel} tone={step.tone} />
                </div>
                <div className="step-note">{step.timingText}</div>
                <div className="step-foot">{step.durationLabel || step.progressLabel || "Без доп. метрик"}</div>
              </div>
            ))}
          </div>
        </SectionCard>

        <SectionCard title="Плеер" subtitle="Preview / audio fallback" action={<MonitorPlay size={16} className="panel-icon" />}>
          {current.player.hasVideoPlayer && current.player.videoSourceURL ? (
            <video className="player-frame" controls src={current.player.videoSourceURL} />
          ) : null}
          {!current.player.hasVideoPlayer && current.player.hasAudioPlayer && current.player.audioPlayerURL ? (
            <audio className="audio-frame" controls src={current.player.audioPlayerURL} />
          ) : null}
          {!current.player.hasVideoPlayer && !current.player.hasAudioPlayer ? (
            <EmptyState text={current.player.playerFallbackText || "Плеер пока недоступен."} />
          ) : null}
          {current.player.previewNotice ? <div className="inline-note">{current.player.previewNotice}</div> : null}
        </SectionCard>
      </div>

      <div className="detail-grid">
        <SectionCard title="Summary" subtitle="Краткий итог по файлу" action={<StatusBadge label={current.summary.statusLabel} tone={current.summary.statusTone} />}>
          {current.summary.hasSummary ? (
            <>
              <div className="text-panel">{current.summary.text}</div>
              {current.summary.highlights.length > 0 ? (
                <div className="chips-row">
                  {current.summary.highlights.map((item) => <span key={item} className="detail-chip">{item}</span>)}
                </div>
              ) : null}
            </>
          ) : (
            <>
              <EmptyState text={current.summary.notice || "Summary пока нет."} />
              {current.summary.showAction ? (
                <button type="button" className="primary-action inline-action" onClick={() => void handleSummaryRequest()}>
                  {current.summary.actionLabel}
                </button>
              ) : null}
            </>
          )}
        </SectionCard>

        <SectionCard title="Transcript" subtitle="Полный текст и сегменты" action={<ScrollText size={16} className="panel-icon" />}>
          {current.transcript.hasTranscript ? (
            <div className="transcript-list">
              {current.transcript.segments.map((segment) => (
                <div key={`${segment.index}-${segment.startLabel}`} className="transcript-item">
                  <span className="timestamp-pill">{segment.startLabel}</span>
                  <div>
                    <div className="table-item-title small">{segment.text}</div>
                    <div className="table-item-subtitle">{segment.endLabel}</div>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <EmptyState text="Транскрипт ещё не готов." />
          )}
        </SectionCard>
      </div>

      <div className="detail-grid">
        <SectionCard title="Triggers и screenshots" subtitle="Результаты анализа триггеров">
          {current.triggers.items.length === 0 ? (
            <EmptyState text={current.triggers.notice || "Триггеры пока не найдены."} />
          ) : (
            <div className="trigger-grid">
              {current.triggers.items.map((item) => (
                <div key={`${item.ruleName}-${item.timestamp}`} className="trigger-card">
                  <div className="step-title-row">
                    <span>{item.ruleName}</span>
                    <StatusBadge label={item.category} tone={current.triggers.statusTone} />
                  </div>
                  <div className="step-note">{item.timestamp} • {item.matchedPhrase}</div>
                  <div className="text-panel compact">{item.segmentText}</div>
                  {item.hasScreenshot && item.screenshotURL ? (
                    <img src={item.screenshotURL} alt={item.ruleName} className="trigger-shot" />
                  ) : (
                    <div className="empty-panel compact">{item.placeholder || "Скриншот пока недоступен"}</div>
                  )}
                </div>
              ))}
            </div>
          )}
        </SectionCard>

        <SectionCard title="Технические детали" subtitle="Snapshot конфигурации и runtime" action={<Settings2 size={16} className="panel-icon" />}>
          <div className="kv-list">
            {current.settingsSnapshot.settings.map((item) => (
              <div key={item.label} className="kv-row">
                <span>{item.label}</span>
                <strong>{item.value}</strong>
              </div>
            ))}
            {current.settingsSnapshot.runtimeSnapshot.map((item) => (
              <div key={item.label} className="kv-row">
                <span>{item.label}</span>
                <strong>{item.value}</strong>
              </div>
            ))}
          </div>
        </SectionCard>
      </div>
    </div>
  );
}
