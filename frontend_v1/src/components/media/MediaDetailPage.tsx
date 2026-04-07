import { useCallback, useRef, useState } from "react";
import { useParams, useNavigate, Link } from "react-router-dom";
import { ArrowLeft, Loader2, AlertCircle, Trash2 } from "lucide-react";
import { api } from "../../api/client";
import type { MediaDetailResponse } from "../../models/types";
import { usePolling } from "../../hooks/usePolling";
import { useMediaPlayer } from "../../hooks/useMediaPlayer";
import { StatusChip } from "../ui/StatusChip";
import { Button } from "../ui/Button";
import { EmptyState } from "../ui/EmptyState";
import { SummaryCard } from "./SummaryCard";
import { PlayerArea } from "./PlayerArea";
import { TranscriptViewer } from "./TranscriptViewer";
import { TriggerMatches } from "./TriggerMatches";
import { TechDetails } from "./TechDetails";

export function MediaDetailPage() {
  const { mediaId = "" } = useParams();
  const navigate = useNavigate();

  const fetcher = useCallback(() => api.mediaDetail(mediaId), [mediaId]);
  const { data, error, loading, refresh } = usePolling<MediaDetailResponse>(
    fetcher,
    4000,
    true
  );

  const isRunning = data?.pipeline.statusTone === "running";

  // Re-create polling with correct enabled flag by using the hook at top level
  // The polling hook already handles enabled internally, but we always poll
  // and rely on the interval. This is fine — the server call is lightweight.

  const mediaRef = useRef<HTMLMediaElement>(null);
  const { currentTime, seek } = useMediaPlayer(mediaRef);

  const [deleteConfirm, setDeleteConfirm] = useState(false);
  const [deleting, setDeleting] = useState(false);

  async function handleRequestSummary() {
    if (!data) return;
    try {
      await api.requestSummary(data.summary.requestSummaryUrl);
      refresh();
    } catch {
      // Silently fail — polling will pick up changes
    }
  }

  async function handleDelete() {
    if (!data) return;
    setDeleting(true);
    try {
      await api.deleteMedia(data.media.id);
      navigate("/");
    } catch {
      setDeleting(false);
      setDeleteConfirm(false);
    }
  }

  if (loading && !data) {
    return (
      <div
        style={{
          display: "flex",
          justifyContent: "center",
          alignItems: "center",
          padding: "var(--sp-10)",
        }}
      >
        <Loader2
          size={28}
          style={{ animation: "spin 1s linear infinite", color: "var(--text-muted)" }}
        />
      </div>
    );
  }

  if (error && !data) {
    return (
      <div style={{ padding: "var(--sp-5)" }}>
        <EmptyState text={error} icon={<AlertCircle size={22} />} />
      </div>
    );
  }

  if (!data) return null;

  const { media, pipeline, player, transcript, triggers, summary, settingsSnapshot } = data;

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        gap: "var(--sp-5)",
        maxWidth: 1100,
        margin: "0 auto",
        padding: "var(--sp-5)",
      }}
    >
      {/* Page header */}
      <div>
        <Link
          to="/"
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: "var(--sp-1)",
            color: "var(--text-muted)",
            fontSize: "var(--text-sm)",
            textDecoration: "none",
            marginBottom: "var(--sp-3)",
          }}
        >
          <ArrowLeft size={14} />
          Back
        </Link>

        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "var(--sp-3)",
            flexWrap: "wrap",
          }}
        >
          <h1
            style={{
              fontSize: "var(--text-xl)",
              fontWeight: 700,
              color: "var(--text)",
              margin: 0,
            }}
          >
            {media.name}
          </h1>
          <StatusChip label={pipeline.statusLabel} tone={pipeline.statusTone} />
        </div>

        <p
          style={{
            color: "var(--text-muted)",
            fontSize: "var(--text-sm)",
            marginTop: "var(--sp-1)",
            marginBottom: 0,
          }}
        >
          {media.sizeHuman} &middot; {media.createdAtUtc} &middot; {media.mimeType}
        </p>
      </div>

      {/* Summary */}
      <SummaryCard summary={summary} onRequestSummary={handleRequestSummary} />

      {/* Player + Transcript side-by-side */}
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fit, minmax(400px, 1fr))",
          gap: "var(--sp-5)",
        }}
      >
        <PlayerArea player={player} mediaRef={mediaRef} />
        <TranscriptViewer
          transcript={transcript}
          currentTime={currentTime}
          onSeek={seek}
        />
      </div>

      {/* Trigger matches */}
      <TriggerMatches triggers={triggers} onSeek={seek} />

      {/* Technical details */}
      <TechDetails pipeline={pipeline} settingsSnapshot={settingsSnapshot} />

      {/* Delete action */}
      <div
        style={{
          borderTop: "1px solid var(--border)",
          paddingTop: "var(--sp-4)",
          display: "flex",
          alignItems: "center",
          gap: "var(--sp-3)",
        }}
      >
        {!deleteConfirm ? (
          <Button
            variant="danger"
            icon={<Trash2 size={14} />}
            onClick={() => setDeleteConfirm(true)}
          >
            Delete media
          </Button>
        ) : (
          <>
            <span
              style={{
                color: "var(--error)",
                fontSize: "var(--text-sm)",
                fontWeight: 500,
              }}
            >
              Are you sure? This cannot be undone.
            </span>
            <Button
              variant="danger"
              loading={deleting}
              onClick={handleDelete}
            >
              Confirm delete
            </Button>
            <Button
              variant="ghost"
              onClick={() => setDeleteConfirm(false)}
              disabled={deleting}
            >
              Cancel
            </Button>
          </>
        )}
      </div>
    </div>
  );
}
