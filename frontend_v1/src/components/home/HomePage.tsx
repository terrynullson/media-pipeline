import { useCallback, useEffect, useMemo, useState } from "react";
import { api } from "../../api/client";
import type { UIConfigResponse } from "../../models/types";
import { usePolling } from "../../hooks/usePolling";
import { UploadZone } from "./UploadZone";
import { ActiveJob } from "./ActiveJob";
import { MediaList } from "./MediaList";

export function HomePage() {
  const [config, setConfig] = useState<UIConfigResponse | null>(null);

  useEffect(() => {
    api.uiConfig().then(setConfig).catch(() => undefined);
  }, []);

  const { data: items, loading, refresh } = usePolling(api.media, 5000, true);

  const mediaItems = items ?? [];

  const activeItem = useMemo(
    () => mediaItems.find((i) => i.statusTone === "running"),
    [mediaItems],
  );

  const onUploaded = useCallback(() => {
    refresh();
  }, [refresh]);

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

      {!loading && <MediaList items={mediaItems} onDeleted={refresh} />}
    </div>
  );
}
