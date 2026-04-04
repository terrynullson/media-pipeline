import { useEffect, useMemo, useState } from "react";
import { Search } from "lucide-react";
import { api } from "../lib/api";
import { mockJobs } from "../lib/mocks";
import type { JobItem } from "../lib/types";
import { Card, EmptyState, SectionHeader, StatusBadge } from "../shared/ui";

export function JobsPage() {
  const [items, setItems] = useState<JobItem[]>(mockJobs);
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState("all");

  useEffect(() => {
    api.jobs().then(setItems).catch(() => setItems(mockJobs));
  }, []);

  const filtered = useMemo(
    () =>
      items.filter((item) => {
        const matchesQuery =
          item.mediaName.toLowerCase().includes(query.toLowerCase()) ||
          item.typeLabel.toLowerCase().includes(query.toLowerCase());
        const matchesStatus = status === "all" || item.status === status;
        return matchesQuery && matchesStatus;
      }),
    [items, query, status]
  );

  return (
    <div className="page-stack">
      <SectionHeader eyebrow="Operations" title="Jobs" description="Очередь media-pipeline в list-table оболочке, близкой к референсу." />

      <Card>
        <div className="toolbar">
          <label className="search-field">
            <Search size={14} />
            <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search by media or stage" />
          </label>
          <div className="tabs-row">
            {["all", "pending", "running", "done", "failed"].map((item) => (
              <button key={item} type="button" className={`tab-chip${status === item ? " active" : ""}`} onClick={() => setStatus(item)}>
                {item}
              </button>
            ))}
          </div>
        </div>

        {filtered.length === 0 ? (
          <EmptyState text="Подходящих job пока нет." />
        ) : (
          <div className="data-table">
            <div className="table-head">
              <span>Stage</span>
              <span>Media</span>
              <span>Status</span>
              <span>Progress</span>
              <span>Created</span>
            </div>
            {filtered.map((item) => (
              <a key={item.id} className="table-row" href={`/app/media/${item.mediaId}`}>
                <div>
                  <div className="table-primary">{item.typeLabel}</div>
                  <div className="table-secondary">{item.mediaStageLabel}</div>
                </div>
                <div>
                  <div className="table-primary">{item.mediaName}</div>
                  <div className="table-secondary">{item.mediaStatusLabel}</div>
                </div>
                <div>
                  <StatusBadge label={item.statusLabel} tone={item.statusTone} />
                </div>
                <div>
                  <div className="progress-track">
                    <div className="progress-fill" style={{ width: `${item.progressPercent ?? 0}%` }} />
                  </div>
                  <div className="table-secondary">{item.progressLabel || item.durationLabel || "No progress yet"}</div>
                </div>
                <div className="table-secondary">{item.createdAtUtc}</div>
              </a>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
