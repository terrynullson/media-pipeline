import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { api } from "../lib/api";
import type { MediaDetailResponse } from "../lib/types";
import { Card, EmptyState, SectionHeader, StatusBadge } from "../shared/ui";

export function MediaDetailPage() {
  const { mediaId = "" } = useParams();
  const [data, setData] = useState<MediaDetailResponse | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    api.mediaDetail(mediaId).then(setData).catch(() => setError("Не удалось загрузить детали медиа через API."));
  }, [mediaId]);

  if (error) {
    return <div className="page-stack"><Card><EmptyState text={error} /></Card></div>;
  }

  if (!data) {
    return <div className="page-stack"><Card><EmptyState text="Загружаем media details..." /></Card></div>;
  }

  return (
    <div className="page-stack">
      <SectionHeader
        eyebrow="Media details"
        title={data.media.name}
        description={`${data.media.sizeHuman} · ${data.media.createdAtUtc}`}
        actions={<StatusBadge label={data.pipeline.statusLabel} tone={data.pipeline.statusTone} />}
      />

      <div className="detail-grid">
        <Card title="Pipeline" subtitle={data.pipeline.stageLabel}>
          <div className="detail-meta">
            <div className="meta-item">
              <span className="meta-label">Current stage</span>
              <span className="meta-value">{data.pipeline.currentStage}</span>
            </div>
            <div className="meta-item">
              <span className="meta-label">Progress</span>
              <span className="meta-value">{data.pipeline.stageValue} / {data.pipeline.stageTotal}</span>
            </div>
          </div>
          <div className="step-stack">
            {data.pipeline.steps.map((step) => (
              <div key={step.label} className="step-row">
                <div>
                  <div className="table-primary">{step.label}</div>
                  <div className="table-secondary">{step.timingText}</div>
                </div>
                <StatusBadge label={step.statusLabel} tone={step.tone} />
              </div>
            ))}
          </div>
        </Card>

        <Card title="Playback" subtitle="Preview / audio fallback">
          {data.player.hasVideoPlayer && data.player.videoSourceURL ? (
            <video className="player-frame" controls src={data.player.videoSourceURL} />
          ) : data.player.hasAudioPlayer && data.player.audioPlayerURL ? (
            <audio className="audio-frame" controls src={data.player.audioPlayerURL} />
          ) : (
            <EmptyState text={data.player.playerFallbackText || "Плеер пока недоступен."} />
          )}
          {data.player.previewNotice ? <p className="inline-note">{data.player.previewNotice}</p> : null}
        </Card>
      </div>

      <div className="detail-grid">
        <Card title="Summary" subtitle="Worker output" aside={<StatusBadge label={data.summary.statusLabel} tone={data.summary.statusTone} />}>
          {data.summary.hasSummary ? (
            <>
              <p className="callout-text">{data.summary.text}</p>
              {data.summary.highlights.length > 0 ? (
                <div className="chips-row">
                  {data.summary.highlights.map((item) => <span key={item} className="signal-pill ready">{item}</span>)}
                </div>
              ) : null}
            </>
          ) : (
            <EmptyState text={data.summary.notice || "Summary пока нет."} />
          )}
        </Card>

        <Card title="Transcript" subtitle="Full text / segments">
          {data.transcript.hasTranscript ? (
            <div className="transcript-stack">
              {data.transcript.segments.map((segment) => (
                <div key={`${segment.index}-${segment.startLabel}`} className="transcript-row">
                  <div className="timestamp-pill">{segment.startLabel}</div>
                  <div>
                    <div className="table-primary">{segment.text}</div>
                    <div className="table-secondary">{segment.endLabel}</div>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <EmptyState text="Transcript ещё не готов." />
          )}
        </Card>
      </div>

      <div className="detail-grid">
        <Card title="Triggers" subtitle="Rules / screenshots" aside={<StatusBadge label={data.triggers.statusLabel} tone={data.triggers.statusTone} />}>
          {data.triggers.items.length === 0 ? (
            <EmptyState text={data.triggers.notice || "Триггеры пока не найдены."} />
          ) : (
            <div className="trigger-grid">
              {data.triggers.items.map((item) => (
                <div key={`${item.ruleName}-${item.timestamp}`} className="trigger-card">
                  <div className="trigger-head">
                    <div>
                      <div className="table-primary">{item.ruleName}</div>
                      <div className="table-secondary">{item.timestamp} · {item.category}</div>
                    </div>
                    <span className="signal-pill ready">{item.matchedPhrase}</span>
                  </div>
                  <p className="callout-text">{item.segmentText}</p>
                  {item.hasScreenshot && item.screenshotURL ? (
                    <img className="trigger-shot" src={item.screenshotURL} alt={item.ruleName} />
                  ) : (
                    <div className="empty-state compact">{item.placeholder || "Screenshot недоступен"}</div>
                  )}
                </div>
              ))}
            </div>
          )}
        </Card>

        <Card title="Technical details" subtitle="Runtime snapshot">
          <div className="kv-stack">
            {data.settingsSnapshot.settings.map((item) => (
              <div key={item.label} className="kv-row">
                <span className="meta-label">{item.label}</span>
                <span className="meta-value">{item.value}</span>
              </div>
            ))}
            {data.settingsSnapshot.runtimeSnapshot.map((item) => (
              <div key={item.label} className="kv-row">
                <span className="meta-label">{item.label}</span>
                <span className="meta-value">{item.value}</span>
              </div>
            ))}
          </div>
        </Card>
      </div>
    </div>
  );
}
