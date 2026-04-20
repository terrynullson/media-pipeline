import { useCallback, useMemo, useRef, useState } from "react";
import type { ChangeEvent, DragEvent } from "react";
import { UploadCloud, AlertCircle, CheckCircle, X } from "lucide-react";
import type { UIConfigResponse, UploadProgress } from "../../models/types";
import { api } from "../../api/client";
import { useTranslation } from "../../i18n";
import { Progress } from "../ui/Progress";

interface UploadZoneProps {
  config: UIConfigResponse | null;
  onUploaded: () => void;
}

interface QueueItem {
  id: string;
  file: File;
  status: "queued" | "uploading" | "done" | "error";
  progress: UploadProgress | null;
  error: string | null;
  startedAtMs: number;
}

const DEFAULT_ACCEPT = ".mp4,.mov,.mkv,.avi,.webm,.mp3,.wav,.m4a,.aac,.flac";

const zoneBase: React.CSSProperties = {
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  gap: "var(--sp-3)",
  height: 100,
  borderWidth: 2,
  borderStyle: "dashed",
  borderColor: "var(--border-strong)",
  borderRadius: "var(--radius-md)",
  background: "transparent",
  cursor: "pointer",
  transition:
    "border-color var(--duration-fast) var(--ease), background var(--duration-fast) var(--ease)",
  padding: "0 var(--sp-4)",
};

const zoneDragOver: React.CSSProperties = {
  borderColor: "var(--accent)",
  background: "var(--accent-soft)",
};

const zoneBusy: React.CSSProperties = {
  borderStyle: "solid",
  borderColor: "var(--border)",
};

function formatRemaining(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return "";
  if (seconds < 60) return `Осталось ~${Math.max(1, Math.round(seconds))} сек`;
  const minutes = Math.floor(seconds / 60);
  const restSeconds = Math.round(seconds % 60);
  if (minutes < 60) return `Осталось ~${minutes} мин ${restSeconds} сек`;
  const hours = Math.floor(minutes / 60);
  const restMinutes = minutes % 60;
  return `Осталось ~${hours} ч ${restMinutes} мин`;
}

function humanSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 Б";
  const units = ["Б", "КБ", "МБ", "ГБ", "ТБ"];
  let value = bytes;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  const digits = value >= 10 || unitIndex === 0 ? 0 : 1;
  return `${value.toFixed(digits)} ${units[unitIndex]}`;
}

export function UploadZone({ config, onUploaded }: UploadZoneProps) {
  const { t } = useTranslation();
  const [dragOver, setDragOver] = useState(false);
  const [queue, setQueue] = useState<QueueItem[]>([]);
  const inputRef = useRef<HTMLInputElement>(null);
  const cancelMapRef = useRef<Record<string, () => void>>({});

  const accept = config?.acceptedFormats.join(",") ?? DEFAULT_ACCEPT;
  const acceptedFormats = useMemo(
    () => new Set((config?.acceptedFormats ?? DEFAULT_ACCEPT.split(",")).map((item) => item.toLowerCase())),
    [config],
  );
  const maxUploadBytes = config?.maxUploadBytes ?? 0;

  const activeItems = queue.filter((item) => item.status === "queued" || item.status === "uploading");
  const busy = activeItems.length > 0;

  const aggregate = useMemo(() => {
    if (activeItems.length === 0) return null;

    const total = activeItems.reduce((sum, item) => sum + (item.progress?.total ?? item.file.size), 0);
    const loaded = activeItems.reduce((sum, item) => sum + (item.progress?.loaded ?? 0), 0);
    const percent = total > 0 ? Math.round((loaded / total) * 100) : 0;
    const startedAtMs = Math.min(...activeItems.map((item) => item.startedAtMs));
    const elapsedSec = Math.max(0.001, (Date.now() - startedAtMs) / 1000);
    const bytesPerSec = loaded / elapsedSec;
    const remainingSec = bytesPerSec > 0 ? Math.max(0, (total - loaded) / bytesPerSec) : 0;

    return {
      loaded,
      total,
      percent,
      remainingLabel: formatRemaining(remainingSec),
    };
  }, [activeItems]);

  const updateItem = useCallback((id: string, updater: (item: QueueItem) => QueueItem) => {
    setQueue((prev) => prev.map((item) => (item.id === id ? updater(item) : item)));
  }, []);

  const startUpload = useCallback((item: QueueItem) => {
    updateItem(item.id, (current) => ({
      ...current,
      status: "uploading",
      progress: { loaded: 0, total: current.file.size, percent: 0 },
    }));

    void api.uploadWithProgress(
      item.file,
      (progress) => {
        updateItem(item.id, (current) => ({ ...current, progress, status: "uploading" }));
      },
      (cancel) => {
        cancelMapRef.current[item.id] = cancel;
      },
    ).then(() => {
      delete cancelMapRef.current[item.id];
      updateItem(item.id, (current) => ({ ...current, status: "done", progress: null, error: null }));
      onUploaded();
      window.setTimeout(() => {
        setQueue((prev) => prev.filter((entry) => entry.id !== item.id || entry.status === "uploading" || entry.status === "queued"));
      }, 3000);
    }).catch((err) => {
      delete cancelMapRef.current[item.id];
      const message = err instanceof Error ? err.message : t("upload.error.generic");
      if (message === "cancelled") {
        setQueue((prev) => prev.filter((entry) => entry.id !== item.id));
        return;
      }
      updateItem(item.id, (current) => ({ ...current, status: "error", progress: null, error: message }));
    });
  }, [onUploaded, t, updateItem]);

  const addFiles = useCallback((files: FileList | File[]) => {
    const arr = Array.from(files);
    if (arr.length === 0) return;

    const startedAtMs = Date.now();
    const prepared: QueueItem[] = arr.map((file, index) => {
      const extension = `.${(file.name.split(".").pop() ?? "").toLowerCase()}`;
      if (!acceptedFormats.has(extension)) {
        return {
          id: `${startedAtMs}-${index}-${file.name}`,
          file,
          status: "error",
          progress: null,
          error: `Недопустимый формат. Разрешено: ${(config?.acceptedFormats ?? Array.from(acceptedFormats)).join(", ")}`,
          startedAtMs,
        };
      }
      if (maxUploadBytes > 0 && file.size > maxUploadBytes) {
        return {
          id: `${startedAtMs}-${index}-${file.name}`,
          file,
          status: "error",
          progress: null,
          error: `Файл слишком большой. Максимум: ${config?.maxUploadHuman ?? humanSize(maxUploadBytes)}`,
          startedAtMs,
        };
      }
      return {
        id: `${startedAtMs}-${index}-${file.name}`,
        file,
        status: "queued",
        progress: { loaded: 0, total: file.size, percent: 0 },
        error: null,
        startedAtMs,
      };
    });

    setQueue((prev) => [...prepared, ...prev]);
    prepared.filter((item) => item.status === "queued").forEach(startUpload);
  }, [acceptedFormats, config, maxUploadBytes, startUpload]);

  const cancelUploads = useCallback(() => {
    Object.values(cancelMapRef.current).forEach((cancel) => cancel());
    cancelMapRef.current = {};
  }, []);

  const onDragOver = useCallback((e: DragEvent) => {
    e.preventDefault();
    setDragOver(true);
  }, []);

  const onDragLeave = useCallback(() => setDragOver(false), []);

  const onDrop = useCallback(
    (e: DragEvent) => {
      e.preventDefault();
      setDragOver(false);
      if (e.dataTransfer.files.length > 0) {
        addFiles(e.dataTransfer.files);
      }
    },
    [addFiles],
  );

  const onFileChange = useCallback(
    (e: ChangeEvent<HTMLInputElement>) => {
      if (e.target.files && e.target.files.length > 0) {
        addFiles(e.target.files);
      }
      if (inputRef.current) inputRef.current.value = "";
    },
    [addFiles],
  );

  const computedStyle: React.CSSProperties = {
    ...zoneBase,
    ...(dragOver && !busy ? zoneDragOver : {}),
    ...(busy ? zoneBusy : {}),
  };

  const finishedItems = queue.filter((item) => item.status === "done" || item.status === "error");

  return (
    <div>
      <label
        style={computedStyle}
        onDragOver={onDragOver}
        onDragLeave={onDragLeave}
        onDrop={onDrop}
      >
        {aggregate ? (
          <div style={{ flex: 1, display: "flex", flexDirection: "column", gap: "var(--sp-2)" }}>
            <div
              style={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                fontSize: "var(--text-sm)",
              }}
            >
              <span
                style={{
                  fontWeight: 600,
                  color: "var(--text)",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                  maxWidth: "60%",
                }}
              >
                Загружается файлов: {activeItems.length}
              </span>
              <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)" }}>
                <span style={{ color: "var(--text-muted)", fontVariantNumeric: "tabular-nums" }}>
                  {aggregate.percent}%
                </span>
                <button
                  type="button"
                  title="Отменить загрузку"
                  onClick={(e) => { e.preventDefault(); cancelUploads(); }}
                  style={{
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    width: 20,
                    height: 20,
                    borderRadius: "50%",
                    border: "none",
                    background: "var(--border-strong)",
                    color: "var(--text-muted)",
                    cursor: "pointer",
                    padding: 0,
                    flexShrink: 0,
                  }}
                >
                  <X size={12} />
                </button>
              </div>
            </div>
            <Progress percent={aggregate.percent} height={6} animate />
            <div style={{ display: "flex", justifyContent: "space-between", gap: "var(--sp-3)", fontSize: "var(--text-xs)", color: "var(--text-muted)" }}>
              <span>{humanSize(aggregate.loaded)} / {humanSize(aggregate.total)}</span>
              <span>{aggregate.remainingLabel}</span>
            </div>
          </div>
        ) : (
          <>
            <UploadCloud size={22} style={{ color: "var(--text-muted)", flexShrink: 0 }} />
            <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
              <span style={{ fontSize: "var(--text-sm)", fontWeight: 600, color: "var(--text)" }}>
                {t("upload.dropMultiple")}
              </span>
              <span style={{ fontSize: "var(--text-xs)", color: "var(--text-muted)" }}>
                {config
                  ? `Max ${config.maxUploadHuman} · ${config.acceptedFormats.join(", ")}`
                  : t("upload.loading")}
              </span>
            </div>
          </>
        )}
        <input
          ref={inputRef}
          type="file"
          hidden
          accept={accept}
          multiple
          onChange={onFileChange}
          disabled={false}
        />
      </label>

      {finishedItems.length > 0 && (
        <div style={{ marginTop: "var(--sp-2)", display: "flex", flexDirection: "column", gap: "var(--sp-1)" }}>
          {finishedItems.map((item) => (
            <div
              key={item.id}
              style={{
                display: "flex",
                alignItems: "center",
                gap: "var(--sp-2)",
                fontSize: "var(--text-sm)",
                animation: "fade-in var(--duration-fast) var(--ease)",
              }}
            >
              {item.status === "done" ? (
                <CheckCircle size={14} style={{ color: "var(--success)", flexShrink: 0 }} />
              ) : (
                <AlertCircle size={14} style={{ color: "var(--error)", flexShrink: 0 }} />
              )}
              <span
                style={{
                  color: item.status === "done" ? "var(--text-muted)" : "var(--error)",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
              >
                {item.file.name}
                {item.error && ` — ${item.error}`}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
