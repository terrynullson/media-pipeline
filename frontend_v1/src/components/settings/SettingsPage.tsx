/**
 * SettingsPage — самостоятельная страница настроек, доступная по роуту /app-v1/settings.
 *
 * Архитектура:
 *   - Загружает настройки через api.settings() при монтировании.
 *   - Сохраняет через api.updateSettings() без перезагрузки страницы.
 *   - TriggerRules вынесен в отдельный компонент и монтируется здесь целиком.
 *   - Страница делится на секции: сейчас «Транскрипция» и «Правила триггеров».
 *     Позже сюда легко добавить «UI», «Рантайм», «Безопасность».
 *
 * Навигация:
 *   - Кнопка «←» в шапке возвращает на dashboard (/).
 *   - URL: /app-v1/settings
 */

import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { ArrowLeft, Settings } from "lucide-react";
import { api } from "../../api/client";
import type { SettingsResponse } from "../../models/types";
import { useTranslation } from "../../i18n";
import { Button } from "../ui/Button";
import { TriggerRules } from "./TriggerRules";

// ─── Вспомогательные стили (единый visual language с остальным приложением) ──

const sectionHeadingStyle: React.CSSProperties = {
  fontSize: "var(--text-sm)",
  fontWeight: 600,
  color: "var(--text-secondary)",
  letterSpacing: "var(--tracking-wide)",
  textTransform: "uppercase",
  marginBottom: "var(--sp-3)",
};

const labelStyle: React.CSSProperties = {
  display: "flex",
  flexDirection: "column",
  gap: 4,
  fontSize: "var(--text-sm)",
  color: "var(--text-muted)",
};

const inputStyle: React.CSSProperties = {
  padding: "7px 10px",
  borderRadius: "var(--radius-sm)",
  border: "1px solid var(--border-strong)",
  background: "var(--bg-inset)",
  color: "var(--text)",
  fontSize: "var(--text-base)",
  outline: "none",
};

// ─── Секция: Транскрипция ─────────────────────────────────────────────────────

interface TranscriptionSectionProps {
  settings: SettingsResponse;
}

function TranscriptionSection({ settings }: TranscriptionSectionProps) {
  const { t } = useTranslation();
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  // uiTheme is intentionally excluded from the editable form: it has no UI
  // control in this section and is managed separately via api.updateUITheme().
  // We pass it through unchanged when saving to avoid overwriting the user's
  // theme preference with a stale default.
  const uiTheme = settings.profile.uiTheme || "new";
  const [form, setForm] = useState({
    backend: settings.profile.backend || "faster-whisper",
    modelName: settings.profile.modelName || "base",
    device: settings.profile.device || "cpu",
    computeType: settings.profile.computeType || "int8",
    language: settings.profile.language || "",
    beamSize: settings.profile.beamSize || 5,
    vadEnabled: settings.profile.vadEnabled ?? true,
  });

  const computeOptions =
    form.device === "cuda"
      ? (settings.options.cuda ?? ["float16"])
      : (settings.options.cpu ?? ["int8", "float32"]);

  async function handleSave() {
    setSaving(true);
    setSaved(false);
    try {
      await api.updateSettings({ ...form, uiTheme });
      setSaved(true);
      setTimeout(() => setSaved(false), 2500);
    } catch {
      // ошибку показываем через предупреждения
    }
    setSaving(false);
  }

  return (
    <section
      style={{
        background: "var(--bg-card)",
        border: "1px solid var(--border)",
        borderRadius: "var(--radius-md)",
        padding: "var(--sp-5)",
      }}
    >
      <h2 style={sectionHeadingStyle}>{t("settings.transcription")}</h2>

      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fill, minmax(180px, 1fr))",
          gap: "var(--sp-4)",
          marginBottom: "var(--sp-4)",
        }}
      >
        <label style={labelStyle}>
          {t("settings.backend")}
          <select
            style={inputStyle}
            value={form.backend}
            onChange={(e) => setForm({ ...form, backend: e.target.value })}
          >
            {settings.options.backends.map((b) => (
              <option key={b} value={b}>
                {b}
              </option>
            ))}
          </select>
        </label>

        <label style={labelStyle}>
          {t("settings.model")}
          <select
            style={inputStyle}
            value={form.modelName}
            onChange={(e) => setForm({ ...form, modelName: e.target.value })}
          >
            {settings.options.models.map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </select>
        </label>

        <label style={labelStyle}>
          {t("settings.device")}
          <select
            style={inputStyle}
            value={form.device}
            onChange={(e) =>
              setForm({
                ...form,
                device: e.target.value,
                computeType: e.target.value === "cuda" ? "float16" : "int8",
              })
            }
          >
            {settings.options.devices.map((d) => (
              <option key={d} value={d}>
                {d}
              </option>
            ))}
          </select>
        </label>

        <label style={labelStyle}>
          {t("settings.computeType")}
          <select
            style={inputStyle}
            value={form.computeType}
            onChange={(e) => setForm({ ...form, computeType: e.target.value })}
          >
            {computeOptions.map((c) => (
              <option key={c} value={c}>
                {c}
              </option>
            ))}
          </select>
        </label>

        <label style={labelStyle}>
          {t("settings.language")}
          <input
            style={inputStyle}
            value={form.language}
            onChange={(e) => setForm({ ...form, language: e.target.value })}
            placeholder={t("settings.auto")}
          />
        </label>

        <label style={labelStyle}>
          {t("settings.beamSize")}
          <input
            style={inputStyle}
            type="number"
            min={1}
            max={10}
            value={form.beamSize}
            onChange={(e) =>
              setForm({ ...form, beamSize: Number(e.target.value) })
            }
          />
        </label>
      </div>

      <label
        style={{
          ...labelStyle,
          flexDirection: "row",
          alignItems: "center",
          gap: 8,
          marginBottom: "var(--sp-4)",
        }}
      >
        <input
          type="checkbox"
          checked={form.vadEnabled}
          onChange={(e) => setForm({ ...form, vadEnabled: e.target.checked })}
          style={{ accentColor: "var(--accent)" }}
        />
        {t("settings.vadFilter")}
      </label>

      {settings.warnings.length > 0 && (
        <div
          style={{
            marginBottom: "var(--sp-4)",
            padding: "var(--sp-2) var(--sp-3)",
            borderRadius: "var(--radius-sm)",
            background: "var(--warning-soft)",
            color: "var(--warning)",
            fontSize: "var(--text-sm)",
          }}
        >
          {settings.warnings.join(". ")}
        </div>
      )}

      <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-3)" }}>
        <Button variant="primary" size="sm" loading={saving} onClick={handleSave}>
          {t("settings.save")}
        </Button>
        {saved && (
          <span style={{ fontSize: "var(--text-sm)", color: "var(--success)" }}>
            {t("settings.saved")}
          </span>
        )}
      </div>
    </section>
  );
}

// ─── Секция: Правила триггеров ────────────────────────────────────────────────

function TriggerRulesSection() {
  const { t } = useTranslation();
  return (
    <section
      style={{
        background: "var(--bg-card)",
        border: "1px solid var(--border)",
        borderRadius: "var(--radius-md)",
        padding: "var(--sp-5)",
      }}
    >
      <h2 style={sectionHeadingStyle}>{t("rules.title")}</h2>
      <TriggerRules />
    </section>
  );
}

// ─── Главный компонент страницы ───────────────────────────────────────────────

export function SettingsPage() {
  const { t } = useTranslation();
  const [settings, setSettings] = useState<SettingsResponse | null>(null);
  const [loadError, setLoadError] = useState(false);

  useEffect(() => {
    api
      .settings()
      .then(setSettings)
      .catch(() => setLoadError(true));
  }, []);

  return (
    <div>
      {/* ── Шапка страницы ── */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: "var(--sp-3)",
          marginBottom: "var(--sp-6)",
        }}
      >
        <Link
          to="/"
          style={{
            display: "flex",
            alignItems: "center",
            gap: "var(--sp-1)",
            fontSize: "var(--text-sm)",
            color: "var(--text-muted)",
            textDecoration: "none",
          }}
          aria-label={t("action.back")}
        >
          <ArrowLeft size={15} />
          {t("action.back")}
        </Link>

        <span style={{ color: "var(--border-strong)" }}>/</span>

        <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)" }}>
          <Settings size={16} style={{ color: "var(--text-muted)" }} />
          <h1
            style={{
              fontSize: "var(--text-lg)",
              fontWeight: 700,
              color: "var(--text)",
            }}
          >
            {t("settings.title")}
          </h1>
        </div>
      </div>

      {/* ── Тело страницы ── */}
      {loadError && (
        <div
          style={{
            padding: "var(--sp-4)",
            borderRadius: "var(--radius-md)",
            background: "var(--error-soft)",
            color: "var(--error)",
            fontSize: "var(--text-sm)",
          }}
        >
          {t("settings.loadError")}
        </div>
      )}

      {!settings && !loadError && (
        <div style={{ color: "var(--text-muted)", fontSize: "var(--text-sm)" }}>
          {t("settings.loading")}
        </div>
      )}

      {settings && (
        <div style={{ display: "flex", flexDirection: "column", gap: "var(--sp-6)" }}>
          <TranscriptionSection settings={settings} />
          <TriggerRulesSection />
          {/*
            Здесь можно добавлять новые секции по мере роста:
            <UISettingsSection />
            <RuntimeSettingsSection />
            <AdminSection />
          */}
        </div>
      )}
    </div>
  );
}
