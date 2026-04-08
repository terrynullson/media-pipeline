import { useState, useCallback, useRef } from "react";

export function useMediaPlayer() {
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [playing, setPlaying] = useState(false);
  const elementRef = useRef<HTMLMediaElement | null>(null);
  const cleanupRef = useRef<(() => void) | null>(null);

  const mediaRef = useCallback((node: HTMLMediaElement | null) => {
    // Cleanup previous element's listeners
    if (cleanupRef.current) {
      cleanupRef.current();
      cleanupRef.current = null;
    }

    elementRef.current = node;
    if (!node) return;

    let lastSecond = -1;

    const onTimeUpdate = () => {
      const sec = Math.floor(node.currentTime);
      if (sec !== lastSecond) {
        lastSecond = sec;
        setCurrentTime(node.currentTime);
      }
    };

    const onMeta = () => setDuration(node.duration);
    const onPlay = () => setPlaying(true);
    const onPause = () => setPlaying(false);

    node.addEventListener("timeupdate", onTimeUpdate);
    node.addEventListener("loadedmetadata", onMeta);
    node.addEventListener("play", onPlay);
    node.addEventListener("pause", onPause);

    if (node.duration) setDuration(node.duration);

    cleanupRef.current = () => {
      node.removeEventListener("timeupdate", onTimeUpdate);
      node.removeEventListener("loadedmetadata", onMeta);
      node.removeEventListener("play", onPlay);
      node.removeEventListener("pause", onPause);
    };
  }, []);

  const seek = useCallback((time: number) => {
    const el = elementRef.current;
    if (!el) return;
    el.currentTime = time;
    if (el.paused) el.play().catch(() => {});
  }, []);

  return { currentTime, duration, playing, seek, mediaRef };
}
