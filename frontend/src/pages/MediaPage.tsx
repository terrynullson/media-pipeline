import { useEffect, useMemo, useState } from "react";
import { Search } from "lucide-react";
import { api } from "../lib/api";
import { mockMedia } from "../lib/mocks";
import type { MediaListItem } from "../lib/types";
import { Card, EmptyState, SectionHeader, StatusBadge } from "../shared/ui";

export function MediaPage() {
  const [items, setItems] = useState<MediaListItem[]>(mockMedia);
  const [query, setQuery] = useState("");

  useEffect(() => {
    api.media().then(setItems).catch(() => setItems(mockMedia));
  }, []);

  const filtered = useMemo(() => items.filter((item) => item.name.toLowerCase().includes(query.toLowerCase())), [items, query]);

  return (
    <div className="page-stack">
      <SectionHeader eyebrow="Library" title="Media / Assets" description="Список материалов в том же visual shell: плотные строки, мягкие границы и рабочая тёмная иерархия." />

      <Card>
        <div className="toolbar compact">
          <label className="search-field">
            <Search size={14} />
            <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search files" />
          </label>
        </div>

        {filtered.length === 0 ? (
          <EmptyState text="Media список пока пуст." />
        ) : (
          <div className="data-table">
            <div className="table-head media-head">
              <span>Name</span>
              <span>Type</span>
              <span>Created</span>
              <span>Pipeline</span>
              <span>Signals</span>
            </div>
            {filtered.map((item) => (
              <a key={item.id} className="table-row media-row" href={`/app/media/${item.id}`}>
                <div>
                  <div className="table-primary">{item.name}</div>
                  <div className="table-secondary">{item.sizeHuman}</div>
                </div>
                <div className="table-secondary">{item.isAudioOnly ? "Audio" : "Video"}</div>
                <div className="table-secondary">{item.createdAtUtc}</div>
                <div>
                  <StatusBadge label={item.statusLabel} tone={item.statusTone} />
                  <div className="table-secondary">{item.currentStage}</div>
                </div>
                <div className="signal-row">
                  <span className={`signal-pill${item.previewReady ? " ready" : ""}`}>Preview</span>
                  <span className={`signal-pill${item.hasTranscript ? " ready" : ""}`}>Transcript</span>
                </div>
              </a>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
