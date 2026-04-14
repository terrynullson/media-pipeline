import { Settings, Waves, Sun, Moon } from "lucide-react";
import { useTranslation, type Locale } from "../../i18n";
import { useTheme } from "../../theme";
import { api } from "../../api/client";
import { usePolling } from "../../hooks/usePolling";
import type { WorkerStatusResponse } from "../../models/types";

interface TopbarProps {
  onSettingsClick: () => void;
}

const iconBtn: React.CSSProperties = {
  width: 32,
  height: 32,
  borderRadius: "var(--radius-sm)",
  display: "grid",
  placeItems: "center",
  color: "var(--text-muted)",
  border: "1px solid var(--border)",
  background: "none",
  cursor: "pointer",
  transition: "color var(--duration-fast) var(--ease), border-color var(--duration-fast) var(--ease), background var(--duration-fast) var(--ease)",
};

function hoverIn(e: React.MouseEvent<HTMLButtonElement>) {
  e.currentTarget.style.color = "var(--text-secondary)";
  e.currentTarget.style.borderColor = "var(--border-strong)";
}

function hoverOut(e: React.MouseEvent<HTMLButtonElement>) {
  e.currentTarget.style.color = "var(--text-muted)";
  e.currentTarget.style.borderColor = "var(--border)";
}

function WorkerStatusChip({ status }: { status: WorkerStatusResponse | null | undefined }) {
  if (!status) return null;

  const { likelyAlive, currentJob, queue } = status;
  const hasWork = currentJob || queue.pending > 0;

  let dot: string;
  let label: string;

  if (!hasWork && !likelyAlive) {
    dot = "var(--text-muted)";
    label = "Воркер простаивает";
  } else if (!likelyAlive) {
    dot = "var(--error)";
    label = "Воркер не отвечает";
  } else if (currentJob) {
    dot = "var(--success)";
    const pct = currentJob.progressPercent != null ? ` · ${currentJob.progressPercent}%` : "";
    label = `Воркер активен${pct}`;
  } else {
    dot = "var(--warning, #ca8a04)";
    label = `В очереди: ${queue.pending}`;
  }

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 5,
        fontSize: "var(--text-xs)",
        color: "var(--text-muted)",
        whiteSpace: "nowrap",
      }}
      title={currentJob ? `${currentJob.type} · media ${currentJob.mediaId}` : `Ожидает: ${queue.pending}`}
    >
      <span
        style={{
          width: 7,
          height: 7,
          borderRadius: "50%",
          background: dot,
          flexShrink: 0,
          boxShadow: likelyAlive && hasWork ? `0 0 5px ${dot}` : "none",
        }}
      />
      <span>{label}</span>
    </div>
  );
}

export function Topbar({ onSettingsClick }: TopbarProps) {
  const { t, locale, setLocale } = useTranslation();
  const { theme, toggleTheme } = useTheme();
  const { data: workerStatus } = usePolling(api.workerStatus, 5000, true);

  const nextLocale: Locale = locale === "ru" ? "en" : "ru";

  return (
    <header
      style={{
        position: "sticky",
        top: 0,
        zIndex: "var(--z-sticky)" as unknown as number,
        background: "var(--bg-surface)",
        borderBottom: "1px solid var(--border)",
      }}
    >
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: "var(--sp-4)",
          height: 48,
          maxWidth: 1120,
          margin: "0 auto",
          padding: "0 var(--sp-6)",
        }}
      >
        {/* Left: logo */}
        <a
          href="/app-v1/"
          style={{
            display: "flex",
            alignItems: "center",
            gap: "var(--sp-2)",
          }}
        >
          <div
            style={{
              width: 26,
              height: 26,
              borderRadius: "var(--radius-sm)",
              background: "var(--accent-soft)",
              display: "grid",
              placeItems: "center",
              color: "var(--accent)",
            }}
          >
            <Waves size={14} />
          </div>
          <span
            style={{
              fontSize: "var(--text-base)",
              fontWeight: 700,
              letterSpacing: "var(--tracking-tight)",
              color: "var(--text)",
            }}
          >
            {t("app.title")}
          </span>
        </a>

        {/* Center: theme toggle */}
        <button
          onClick={toggleTheme}
          aria-label="Toggle theme"
          style={{
            ...iconBtn,
            position: "absolute",
            left: "50%",
            transform: "translateX(-50%)",
            borderColor: "var(--border-accent)",
            color: "var(--accent)",
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.background = "var(--accent-soft)";
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.background = "none";
          }}
        >
          {theme === "dark" ? <Sun size={15} /> : <Moon size={15} />}
        </button>

        {/* Right: worker status + lang + settings */}
        <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-3)" }}>
          <WorkerStatusChip status={workerStatus} />
          <button
            onClick={() => setLocale(nextLocale)}
            aria-label="Switch language"
            style={iconBtn}
            onMouseEnter={hoverIn}
            onMouseLeave={hoverOut}
          >
            <span style={{ fontSize: "var(--text-xs)", fontWeight: 700, letterSpacing: "0.02em" }}>
              {locale === "ru" ? "EN" : "RU"}
            </span>
          </button>

          <button
            onClick={onSettingsClick}
            aria-label={t("topbar.settings")}
            style={iconBtn}
            onMouseEnter={hoverIn}
            onMouseLeave={hoverOut}
          >
            <Settings size={15} />
          </button>
        </div>
      </div>
    </header>
  );
}
