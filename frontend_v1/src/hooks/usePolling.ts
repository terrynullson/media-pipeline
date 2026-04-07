import { useEffect, useRef, useState } from "react";

export function usePolling<T>(
  fetcher: () => Promise<T>,
  intervalMs: number,
  enabled: boolean
): { data: T | null; error: string | null; loading: boolean; refresh: () => void } {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const inFlight = useRef(false);
  const fetcherRef = useRef(fetcher);
  fetcherRef.current = fetcher;

  const doFetch = () => {
    if (inFlight.current) return;
    inFlight.current = true;
    fetcherRef
      .current()
      .then((result) => {
        setData(result);
        setError(null);
      })
      .catch((err) => setError(err instanceof Error ? err.message : "Fetch failed"))
      .finally(() => {
        inFlight.current = false;
        setLoading(false);
      });
  };

  useEffect(() => {
    doFetch();
    if (!enabled) return;
    const id = setInterval(doFetch, intervalMs);
    return () => clearInterval(id);
  }, [intervalMs, enabled]);

  return { data, error, loading, refresh: doFetch };
}
