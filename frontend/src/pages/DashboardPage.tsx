import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { AlertTriangle, ArrowRight, HardDriveUpload, ListVideo, Workflow } from "lucide-react";
import { api } from "../lib/api";
import { mockDashboard } from "../lib/mocks";
import type { DashboardResponse } from "../lib/types";
import { Card, EmptyState, SectionHeader, StatusBadge } from "../shared/ui";

export function DashboardPage() {
  const [data, setData] = useState<DashboardResponse>(mockDashboard);

  useEffect(() => {
    api.dashboard().then(setData).catch(() => setData(mockDashboard));
  }, []);

  return (
    <div className="page-stack">
      <SectionHeader
        eyebrow="Workspace"
        title="Dashboard"
        description="Новый shell повторяет тёмную workspace-оболочку референса и показывает живое состояние media-pipeline."
      />

      <div className="metrics-grid">
        {data.overview.map((item) => (
          <Card key={item.label} className="metric-card">
            <div className="metric-label">{item.label}</div>
            <div className="metric-value">{item.value}</div>
            <div className="metric-help">{item.help}</div>
          </Card>
        ))}
      </div>

      <div className="dashboard-grid">
        <Card title="Worker notice" subtitle="Runtime / queue" aside={<StatusBadge label={data.workerNotice.label} tone={data.workerNotice.tone} />}>
          <p className="callout-text">{data.workerNotice.text}</p>
          <div className="queue-metrics">
            {data.queueBreakdown.map((item) => (
              <div key={item.label} className="queue-metric">
                <div className="queue-metric-label">{item.label}</div>
                <div className="queue-metric-value">{item.value}</div>
              </div>
            ))}
          </div>
        </Card>

        <Card title="Recent media" subtitle="Uploads / processing">
          {data.recentMedia.length === 0 ? (
            <EmptyState text="Пока нет элементов для показа." />
          ) : (
            <div className="list-stack">
              {data.recentMedia.map((item) => (
                <Link key={item.id} to={`/media/${item.id}`} className="list-row">
                  <div className="list-row-main">
                    <div className="list-row-title">{item.name}</div>
                    <div className="list-row-subtitle">
                      {item.sizeHuman} · {item.createdAtUtc}
                    </div>
                  </div>
                  <StatusBadge label={item.statusLabel} tone={item.statusTone} />
                </Link>
              ))}
            </div>
          )}
        </Card>
      </div>

      <div className="dashboard-grid">
        <Card title="Recent jobs" subtitle="Queue stream" aside={<Workflow size={16} className="panel-icon" />}>
          {data.recentJobs.length === 0 ? (
            <EmptyState text="Jobs появятся здесь, когда backend отдаст очередь." />
          ) : (
            <div className="list-stack">
              {data.recentJobs.map((item) => (
                <Link key={item.id} to={`/media/${item.mediaId}`} className="list-row">
                  <div className="list-row-icon">
                    <ListVideo size={14} />
                  </div>
                  <div className="list-row-main">
                    <div className="list-row-title">{item.typeLabel}</div>
                    <div className="list-row-subtitle">
                      {item.mediaName} · {item.createdAtUtc}
                    </div>
                  </div>
                  <StatusBadge label={item.statusLabel} tone={item.statusTone} />
                </Link>
              ))}
            </div>
          )}
        </Card>

        <Card title="Latest errors" subtitle="Worker issues" aside={<AlertTriangle size={16} className="panel-icon" />}>
          {data.latestErrors.length === 0 ? (
            <EmptyState text="Последние ошибки не найдены." />
          ) : (
            <div className="list-stack">
              {data.latestErrors.map((item) => (
                <Link key={item.id} to={`/media/${item.mediaId}`} className="list-row error-row">
                  <div className="list-row-main">
                    <div className="list-row-title">{item.mediaName}</div>
                    <div className="list-row-subtitle">{item.errorMessage || item.typeLabel}</div>
                  </div>
                  <ArrowRight size={14} />
                </Link>
              ))}
            </div>
          )}
        </Card>
      </div>

      <div className="metrics-grid triple">
        <Card title="Upload" subtitle="Source intake" aside={<HardDriveUpload size={16} className="panel-icon" />}>
          <p className="callout-text">Новый UI не меняет backend-логику: файл всё так же отправляется в Go API и ставит job в очередь.</p>
        </Card>
        <Card title="Preview" subtitle="Browser-safe stage">
          <p className="callout-text">В details-screen уже заложены video preview и audio fallback, чтобы не тащить старый HTML-плеер.</p>
        </Card>
        <Card title="Transcription" subtitle="Worker only">
          <p className="callout-text">Статусы и технические снимки настройки читаются через API и показаны в единой оболочке.</p>
        </Card>
      </div>
    </div>
  );
}
