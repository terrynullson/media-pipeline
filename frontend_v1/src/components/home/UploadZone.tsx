import { useCallback, useRef, useState } from "react";
import type { ChangeEvent, DragEvent } from "react";
import { UploadCloud, AlertCircle } from "lucide-react";
import type { UIConfigResponse } from "../../models/types";
import { useUpload } from "../../hooks/useUpload";
import { Progress } from "../ui/Progress";

interface UploadZoneProps {
  config: UIConfigResponse | null;
  onUploaded: () => void;
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

const zoneUploading: React.CSSProperties = {
  cursor: "default",
  borderStyle: "solid",
  borderColor: "var(--border)",
};

export function UploadZone({ config, onUploaded }: UploadZoneProps) {
  const { uploading, progress, error, upload, reset } = useUpload();
  const [dragOver, setDragOver] = useState(false);
  const [fileName, setFileName] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  const accept = config?.acceptedFormats.join(",") ?? DEFAULT_ACCEPT;

  const handleFile = useCallback(
    async (file: File | null) => {
      if (!file || uploading) return;
      reset();
      setFileName(file.name);
      const result = await upload(file);
      if (result) {
        setFileName("");
        onUploaded();
      }
    },
    [uploading, upload, reset, onUploaded],
  );

  const onDragOver = useCallback((e: DragEvent) => {
    e.preventDefault();
    setDragOver(true);
  }, []);

  const onDragLeave = useCallback(() => setDragOver(false), []);

  const onDrop = useCallback(
    (e: DragEvent) => {
      e.preventDefault();
      setDragOver(false);
      void handleFile(e.dataTransfer.files?.[0] ?? null);
    },
    [handleFile],
  );

  const onFileChange = useCallback(
    (e: ChangeEvent<HTMLInputElement>) => {
      void handleFile(e.target.files?.[0] ?? null);
      if (inputRef.current) inputRef.current.value = "";
    },
    [handleFile],
  );

  const computedStyle: React.CSSProperties = {
    ...zoneBase,
    ...(dragOver && !uploading ? zoneDragOver : {}),
    ...(uploading ? zoneUploading : {}),
  };

  return (
    <div>
      <label
        style={computedStyle}
        onDragOver={onDragOver}
        onDragLeave={onDragLeave}
        onDrop={onDrop}
      >
        {uploading ? (
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
                  maxWidth: "70%",
                }}
              >
                {fileName}
              </span>
              <span style={{ color: "var(--text-muted)", fontVariantNumeric: "tabular-nums" }}>
                {progress?.percent ?? 0}%
              </span>
            </div>
            <Progress percent={progress?.percent ?? 0} height={6} animate />
          </div>
        ) : (
          <>
            <UploadCloud size={22} style={{ color: "var(--text-muted)", flexShrink: 0 }} />
            <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
              <span style={{ fontSize: "var(--text-sm)", fontWeight: 600, color: "var(--text)" }}>
                Drop file or click to select
              </span>
              <span style={{ fontSize: "var(--text-xs)", color: "var(--text-muted)" }}>
                {config
                  ? `Max ${config.maxUploadHuman} \u00b7 ${config.acceptedFormats.join(", ")}`
                  : "Loading format info\u2026"}
              </span>
            </div>
          </>
        )}
        <input
          ref={inputRef}
          type="file"
          hidden
          accept={accept}
          onChange={onFileChange}
          disabled={uploading}
        />
      </label>

      {error && (
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "var(--sp-2)",
            marginTop: "var(--sp-2)",
            fontSize: "var(--text-sm)",
            color: "var(--error)",
          }}
        >
          <AlertCircle size={14} />
          <span>{error}</span>
        </div>
      )}
    </div>
  );
}
