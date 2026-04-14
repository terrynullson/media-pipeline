import { useCallback, useRef, useState } from "react";
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
  file: File;
  status: "pending" | "uploading" | "done" | "error";
  progress: UploadProgress | null;
  error: string | null;
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
  cursor: "default",
  borderStyle: "solid",
  borderColor: "var(--border)",
};

export function UploadZone({ config, onUploaded }: UploadZoneProps) {
  const { t } = useTranslation();
  const [dragOver, setDragOver] = useState(false);
  const [queue, setQueue] = useState<QueueItem[]>([]);
  const processingRef = useRef(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const cancelRef = useRef<(() => void) | null>(null);

  const accept = config?.acceptedFormats.join(",") ?? DEFAULT_ACCEPT;
  const busy = queue.some((q) => q.status === "uploading" || q.status === "pending");

  const processQueue = useCallback(async (items: QueueItem[]) => {
    if (processingRef.current) return;
    processingRef.current = true;

    const remaining = [...items];

    for (let i = 0; i < remaining.length; i++) {
      if (remaining[i].status !== "pending") continue;

      remaining[i] = { ...remaining[i], status: "uploading", progress: { loaded: 0, total: remaining[i].file.size, percent: 0 } };
      setQueue([...remaining]);

      try {
        await api.uploadWithProgress(
          remaining[i].file,
          (p) => {
            remaining[i] = { ...remaining[i], progress: p };
            setQueue([...remaining]);
          },
          (cancel) => { cancelRef.current = cancel; }
        );
        cancelRef.current = null;
        remaining[i] = { ...remaining[i], status: "done", progress: null, error: null };
      } catch (err) {
        cancelRef.current = null;
        const msg = err instanceof Error ? err.message : t("upload.error.generic");
        if (msg === "cancelled") {
          // Remove cancelled item from queue silently
          remaining.splice(i, 1);
          i--;
          setQueue([...remaining]);
          continue;
        }
        remaining[i] = {
          ...remaining[i],
          status: "error",
          progress: null,
          error: msg,
        };
      }

      setQueue([...remaining]);
      onUploaded();
    }

    processingRef.current = false;

    // Clear completed items after a delay
    setTimeout(() => {
      setQueue((prev) => prev.filter((q) => q.status === "uploading" || q.status === "pending"));
    }, 3000);
  }, [onUploaded, t]);

  const addFiles = useCallback((files: FileList | File[]) => {
    const arr = Array.from(files);
    if (arr.length === 0) return;

    const newItems: QueueItem[] = arr.map((file) => ({
      file,
      status: "pending" as const,
      progress: null,
      error: null,
    }));

    setQueue((prev) => {
      const merged = [...prev, ...newItems];
      // Start processing if not already running
      if (!processingRef.current) {
        processQueue(merged);
      }
      return merged;
    });
  }, [processQueue]);

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

  const activeItem = queue.find((q) => q.status === "uploading");

  return (
    <div>
      <label
        style={computedStyle}
        onDragOver={onDragOver}
        onDragLeave={onDragLeave}
        onDrop={onDrop}
      >
        {activeItem ? (
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
                {activeItem.file.name}
              </span>
              <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)" }}>
                <span style={{ color: "var(--text-muted)", fontVariantNumeric: "tabular-nums" }}>
                  {activeItem.progress?.percent ?? 0}%
                  {queue.filter((q) => q.status === "pending").length > 0 && (
                    <span style={{ marginLeft: 8, fontSize: "var(--text-xs)" }}>
                      +{queue.filter((q) => q.status === "pending").length}
                    </span>
                  )}
                </span>
                <button
                  type="button"
                  title="Отменить загрузку"
                  onClick={(e) => { e.preventDefault(); cancelRef.current?.(); }}
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
            <Progress percent={activeItem.progress?.percent ?? 0} height={6} animate />
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
                  ? `Max ${config.maxUploadHuman} \u00b7 ${config.acceptedFormats.join(", ")}`
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

      {/* Queue list (done/error items) */}
      {queue.filter((q) => q.status === "done" || q.status === "error").length > 0 && (
        <div style={{ marginTop: "var(--sp-2)", display: "flex", flexDirection: "column", gap: "var(--sp-1)" }}>
          {queue
            .filter((q) => q.status === "done" || q.status === "error")
            .map((q, i) => (
              <div
                key={`${q.file.name}-${i}`}
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: "var(--sp-2)",
                  fontSize: "var(--text-sm)",
                  animation: "fade-in var(--duration-fast) var(--ease)",
                }}
              >
                {q.status === "done" ? (
                  <CheckCircle size={14} style={{ color: "var(--success)", flexShrink: 0 }} />
                ) : (
                  <AlertCircle size={14} style={{ color: "var(--error)", flexShrink: 0 }} />
                )}
                <span
                  style={{
                    color: q.status === "done" ? "var(--text-muted)" : "var(--error)",
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                >
                  {q.file.name}
                  {q.error && ` — ${q.error}`}
                </span>
              </div>
            ))}
        </div>
      )}
    </div>
  );
}
