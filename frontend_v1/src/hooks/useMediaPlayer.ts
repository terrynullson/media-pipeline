import { useEffect, useState, useCallback, type RefObject } from "react";

export function useMediaPlayer(mediaRef: RefObject<HTMLMediaElement | null>) {
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [playing, setPlaying] = useState(false);

  useEffect(() => {
    const el = mediaRef.current;
    if (!el) return;

    let lastSecond = -1;

    const onTimeUpdate = () => {
      const sec = Math.floor(el.currentTime);
      if (sec !== lastSecond) {
        lastSecond = sec;
        setCurrentTime(el.currentTime);
      }
    };

    const onMeta = () => setDuration(el.duration);
    const onPlay = () => setPlaying(true);
    const onPause = () => setPlaying(false);

    el.addEventListener("timeupdate", onTimeUpdate);
    el.addEventListener("loadedmetadata", onMeta);
    el.addEventListener("play", onPlay);
    el.addEventListener("pause", onPause);

    if (el.duration) setDuration(el.duration);

    return () => {
      el.removeEventListener("timeupdate", onTimeUpdate);
      el.removeEventListener("loadedmetadata", onMeta);
      el.removeEventListener("play", onPlay);
      el.removeEventListener("pause", onPause);
    };
  }, [mediaRef]);

  const seek = useCallback(
    (time: number) => {
      const el = mediaRef.current;
      if (!el) return;
      el.currentTime = time;
      if (el.paused) el.play().catch(() => {});
    },
    [mediaRef]
  );

  return { currentTime, duration, playing, seek };
}
