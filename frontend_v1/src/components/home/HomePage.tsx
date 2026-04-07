import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../../api/client";
import type { UIConfigResponse } from "../../models/types";
import { usePolling } from "../../hooks/usePolling";
import { UploadZone } from "./UploadZone";
import { ActiveJob } from "./ActiveJob";
import { MediaList } from "./MediaList";

export function HomePage() {
  const navigate = useNavigate();
  const [config, setConfig] = useState<UIConfigResponse | null>(null);

  // Load UI config once on mount
  useEffect(() => {
    api.uiConfig().then(setConfig).catch(() => undefined);
  }, []);

  // Determine whether there is an active job to drive polling
  const { data: items, loading } = usePolling(api.media, 5000, true);

  const mediaItems = items ?? [];

  const activeItem = useMemo(
    () => mediaItems.find((i) => i.statusTone === "running"),
    [mediaItems],
  );

  const onUploaded = useCallback(
    (mediaId: number) => {
      navigate(`/media/${mediaId}`);
    },
    [navigate],
  );

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        gap: "var(--sp-5)",
      }}
    >
      <UploadZone config={config} onUploaded={onUploaded} />

      {activeItem && <ActiveJob item={activeItem} />}

      {!loading && <MediaList items={mediaItems} />}
    </div>
  );
}
