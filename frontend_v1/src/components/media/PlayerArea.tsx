import { useState } from "react";
import { Volume2 } from "lucide-react";
import type { MediaDetailResponse } from "../../models/types";
import { Button } from "../ui/Button";
import { EmptyState } from "../ui/EmptyState";

interface PlayerAreaProps {
  player: MediaDetailResponse["player"];
  mediaRef: React.RefObject<HTMLMediaElement | null>;
}

export function PlayerArea({ player, mediaRef }: PlayerAreaProps) {
  const [useAudioFallback, setUseAudioFallback] = useState(false);

  const showVideo =
    !useAudioFallback && player.hasVideoPlayer && player.videoSourceURL;
  const showAudio =
    !showVideo && player.hasAudioPlayer && player.audioPlayerURL;
  const showFallbackAudio =
    useAudioFallback && player.hasSecondaryAudioFallback && player.audioFallbackURL;
  const showNothing = !showVideo && !showAudio && !showFallbackAudio;

  return (
    <div>
      {showVideo && (
        <video
          ref={mediaRef as React.RefObject<HTMLVideoElement>}
          controls
          src={player.videoSourceURL}
          style={{
            width: "100%",
            borderRadius: "var(--radius-md)",
            background: "#0a0a0a",
            display: "block",
          }}
        />
      )}

      {showAudio && !showFallbackAudio && (
        <div style={{ padding: "var(--sp-4) 0" }}>
          <audio
            ref={mediaRef as React.RefObject<HTMLAudioElement>}
            controls
            src={player.audioPlayerURL}
            style={{ width: "100%", display: "block" }}
          />
        </div>
      )}

      {showFallbackAudio && (
        <div style={{ padding: "var(--sp-4) 0" }}>
          <audio
            ref={mediaRef as React.RefObject<HTMLAudioElement>}
            controls
            src={player.audioFallbackURL}
            style={{ width: "100%", display: "block" }}
          />
        </div>
      )}

      {showNothing && (
        <EmptyState text={player.playerFallbackText || "Player not available yet."} />
      )}

      {player.hasSecondaryAudioFallback && player.audioFallbackURL && (
        <div style={{ marginTop: "var(--sp-2)" }}>
          <Button
            variant="ghost"
            size="sm"
            icon={<Volume2 size={13} />}
            onClick={() => setUseAudioFallback((v) => !v)}
          >
            {useAudioFallback ? "Switch to video" : "Use audio fallback"}
          </Button>
        </div>
      )}

      {player.previewNotice && (
        <p
          style={{
            color: "var(--text-muted)",
            fontSize: "var(--text-sm)",
            marginTop: "var(--sp-2)",
            marginBottom: 0,
          }}
        >
          {player.previewNotice}
        </p>
      )}
    </div>
  );
}
