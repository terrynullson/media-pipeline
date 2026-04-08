import { Settings, Waves, Sun, Moon, Globe } from "lucide-react";
import { useTranslation, type Locale } from "../../i18n";
import { useTheme } from "../../theme";

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

export function Topbar({ onSettingsClick }: TopbarProps) {
  const { t, locale, setLocale } = useTranslation();
  const { theme, toggleTheme } = useTheme();

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

        {/* Right: lang + settings */}
        <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)" }}>
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
