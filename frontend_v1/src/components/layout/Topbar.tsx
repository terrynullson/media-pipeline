import { Settings, Waves } from "lucide-react";

interface TopbarProps {
  onSettingsClick: () => void;
}

export function Topbar({ onSettingsClick }: TopbarProps) {
  return (
    <header
      style={{
        position: "sticky",
        top: 0,
        zIndex: "var(--z-sticky)" as unknown as number,
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        gap: "var(--sp-4)",
        height: 52,
        padding: "0 var(--sp-6)",
        background: "var(--bg-surface)",
        borderBottom: "1px solid var(--border)",
      }}
    >
      <a
        href="/app-v1/"
        style={{
          display: "flex",
          alignItems: "center",
          gap: "var(--sp-3)",
        }}
      >
        <div
          style={{
            width: 28,
            height: 28,
            borderRadius: "var(--radius-sm)",
            background: "var(--accent-soft)",
            display: "grid",
            placeItems: "center",
            color: "var(--accent)",
          }}
        >
          <Waves size={15} />
        </div>
        <span
          style={{
            fontSize: "var(--text-md)",
            fontWeight: 700,
            letterSpacing: "var(--tracking-tight)",
            color: "var(--text)",
          }}
        >
          Media Pipeline
        </span>
      </a>

      <button
        onClick={onSettingsClick}
        aria-label="Settings"
        style={{
          width: 34,
          height: 34,
          borderRadius: "var(--radius-sm)",
          display: "grid",
          placeItems: "center",
          color: "var(--text-muted)",
          border: "1px solid var(--border)",
          transition: "color var(--duration-fast) var(--ease), border-color var(--duration-fast) var(--ease)",
        }}
        onMouseEnter={(e) => {
          e.currentTarget.style.color = "var(--text-secondary)";
          e.currentTarget.style.borderColor = "var(--border-strong)";
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.color = "var(--text-muted)";
          e.currentTarget.style.borderColor = "var(--border)";
        }}
      >
        <Settings size={15} />
      </button>
    </header>
  );
}
