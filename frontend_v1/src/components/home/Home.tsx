import { ChangeEvent, DragEvent, useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { ArrowRight, FileStack, LoaderCircle, UploadCloud } from "lucide-react";
import { api } from "../../api/client";
import type { MediaListItem, UIConfigResponse } from "../../models/types";
import { EmptyState, SectionCard, StatusBadge } from "../common/ui";

export function Home() {
  const [items, setItems] = useState<MediaListItem[]>([]);
  const [config, setConfig] = useState<UIConfigResponse | null>(null);
  const [error, setError] = useState("");
  const [dragging, setDragging] = useState(false);
  const [uploading, setUploading] = useState(false);
  const navigate = useNavigate();

  useEffect(() => {
    api.media()
      .then(setItems)
      .catch(() => setError("Не удалось загрузить данные для главной страницы."));

    api.uiConfig()
      .then(setConfig)
      .catch(() => undefined);
  }, []);

  const processingItem = useMemo(() => items.find((item) => item.statusTone === "running"), [items]);
  const queueItems = useMemo(() => items.filter((item) => item.statusTone === "queued" || item.status === "queued"), [items]);
  const recentDone = useMemo(() => items.filter((item) => item.statusTone === "success").slice(0, 5), [items]);

  async function submitFile(file: File | null) {
    if (!file) {
      return;
    }

    try {
      setUploading(true);
      const result = await api.uploadMedia(file);
      navigate(`/media/${result.mediaId}`);
    } catch {
      setError("Не удалось загрузить файл через новый интерфейс.");
    } finally {
      setUploading(false);
    }
  }

  function onFileInput(event: ChangeEvent<HTMLInputElement>) {
    void submitFile(event.target.files?.[0] ?? null);
  }

  function onDrop(event: DragEvent<HTMLLabelElement>) {
    event.preventDefault();
    setDragging(false);
    void submitFile(event.dataTransfer.files?.[0] ?? null);
  }

  return (
    <div className="page-column">
      <section className="page-hero">
        <div>
          <span className="page-eyebrow">Операционная панель</span>
          <h2>Загрузка, очередь и последние результаты в одном рабочем экране</h2>
          <p>Без фейковой аналитики: только текущая обработка, ожидающие файлы и быстрый переход к деталям.</p>
        </div>
      </section>

      {error ? <div className="flash-banner inline">{error}</div> : null}

      <div className="home-grid">
        <SectionCard
          title="Загрузка файла"
          subtitle="Drag-and-drop или обычный выбор файла"
          className="upload-card"
          action={<StatusBadge label={uploading ? "Идёт загрузка" : "Готово к приёму"} tone={uploading ? "running" : "queued"} />}
        >
          <label
            className={`upload-zone${dragging ? " dragging" : ""}`}
            onDragOver={(event) => {
              event.preventDefault();
              setDragging(true);
            }}
            onDragLeave={() => setDragging(false)}
            onDrop={onDrop}
          >
            <UploadCloud size={26} />
            <div className="upload-title">Перетащите файл сюда</div>
            <div className="upload-copy">или нажмите, чтобы выбрать файл вручную</div>
            <div className="upload-meta">
              {config ? `Лимит: ${config.maxUploadHuman}. Форматы: ${config.acceptedFormats.join(", ")}` : "Читаем ограничения из backend..."}
            </div>
            <input
              type="file"
              hidden
              accept={config?.acceptedFormats.join(",") ?? ".mp4,.mov,.mkv,.avi,.webm,.mp3,.wav,.m4a,.aac,.flac"}
              onChange={onFileInput}
            />
          </label>
        </SectionCard>

        <SectionCard
          title="Сейчас обрабатывается"
          subtitle="Активный файл в основном pipeline"
          action={processingItem ? <StatusBadge label={processingItem.statusLabel} tone={processingItem.statusTone} /> : undefined}
        >
          {processingItem ? (
            <Link to={`/media/${processingItem.id}`} className="media-highlight">
              <div>
                <div className="media-highlight-name">{processingItem.name}</div>
                <div className="media-highlight-meta">{processingItem.currentStage}</div>
              </div>
              <div className="media-highlight-side">
                <div className="progress-rail">
                  <div className="progress-value" style={{ width: `${processingItem.stagePercent}%` }} />
                </div>
                <div className="media-highlight-time">{processingItem.currentTimingText}</div>
              </div>
            </Link>
          ) : (
            <EmptyState text="Сейчас основной pipeline свободен." />
          )}
        </SectionCard>
      </div>

      <div className="home-grid">
        <SectionCard
          title="В очереди"
          subtitle="Файлы уже загружены, но ещё не начали основную обработку"
          action={<span className="panel-count">{queueItems.length}</span>}
        >
          {queueItems.length === 0 ? (
            <EmptyState text="Очередь сейчас пустая." />
          ) : (
            <div className="table-list">
              {queueItems.map((item, index) => (
                <Link key={item.id} to={`/media/${item.id}`} className="table-item">
                  <div className="table-item-main">
                    <div className="table-item-title">#{index + 1} {item.name}</div>
                    <div className="table-item-subtitle">{item.sizeHuman} • {item.createdAtUtc}</div>
                  </div>
                  <StatusBadge label={item.statusLabel} tone={item.statusTone} />
                </Link>
              ))}
            </div>
          )}
        </SectionCard>

        <SectionCard
          title="Последние обработанные"
          subtitle="Последние 5 завершённых файлов"
          action={<FileStack size={16} className="panel-icon" />}
        >
          {recentDone.length === 0 ? (
            <EmptyState text="Готовых файлов пока нет." />
          ) : (
            <div className="table-list">
              {recentDone.map((item) => (
                <Link key={item.id} to={`/media/${item.id}`} className="table-item">
                  <div className="table-item-main">
                    <div className="table-item-title">{item.name}</div>
                    <div className="table-item-subtitle">{item.createdAtUtc}</div>
                  </div>
                  <div className="table-item-action">
                    <StatusBadge label={item.statusLabel} tone={item.statusTone} />
                    <ArrowRight size={14} />
                  </div>
                </Link>
              ))}
            </div>
          )}
        </SectionCard>
      </div>

      <SectionCard
        title="Обзор состояния"
        subtitle="Короткая сводка по реальным данным backend"
        action={<LoaderCircle size={16} className="panel-icon spinning-soft" />}
      >
        <div className="kpi-row">
          <div className="kpi-card">
            <span>Всего файлов</span>
            <strong>{items.length}</strong>
          </div>
          <div className="kpi-card">
            <span>В работе</span>
            <strong>{processingItem ? 1 : 0}</strong>
          </div>
          <div className="kpi-card">
            <span>В очереди</span>
            <strong>{queueItems.length}</strong>
          </div>
          <div className="kpi-card">
            <span>Готово</span>
            <strong>{recentDone.length}</strong>
          </div>
        </div>
      </SectionCard>
    </div>
  );
}
