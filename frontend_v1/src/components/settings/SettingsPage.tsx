/**
 * SettingsPage — настройки приложения, /app-v1/settings.
 *
 * Макет:
 *   header (breadcrumb "← Назад / ⚙ Настройки")
 *   ┌── sidebar ─────┬── content ─────────────────────┐
 *   │ • Транскрипция │ заголовок секции + описание    │
 *   │ • Правила      │ форма секции                   │
 *   └────────────────┴────────────────────────────────┘
 *
 * Добавить новую секцию:
 *   1. Расширить union Section.
 *   2. Добавить запись в NAV_ITEMS (id, icon, titleKey, descKey).
 *   3. Отрендерить тело в switch внутри SettingsPage.
 *
 * uiTheme намеренно не редактируется здесь — им управляет Topbar через
 * api.updateUITheme(). Значение пробрасывается неизменным при сохранении,
 * чтобы не затереть выбранную пользователем тему.
 */

import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import {
  ArrowLeft,
  Settings as SettingsIcon,
  Cpu,
  ListFilter,
  CheckCircle2,
  AlertTriangle,
  BarChart3,
} from "lucide-react";
import { api } from "../../api/client";
import type { SettingsResponse } from "../../models/types";
import { useTranslation, type TranslationKey } from "../../i18n";
import { Button } from "../ui/Button";
import { TriggerRules } from "./TriggerRules";

// ─── Типы ────────────────────────────────────────────────────────────────────

type Section = "transcription" | "rules" | "analytics";

interface NavItem {
  id: Section;
  icon: React.ReactNode;
  titleKey: TranslationKey;
  descKey: TranslationKey;
}

const NAV_ITEMS: NavItem[] = [
  {
    id: "transcription",
    icon: <Cpu size={15} />,
    titleKey: "settings.transcription",
    descKey: "settings.transcriptionDesc",
  },
  {
    id: "rules",
    icon: <ListFilter size={15} />,
    titleKey: "rules.title",
    descKey: "settings.rulesDesc",
  },
  {
    id: "analytics",
    icon: <BarChart3 size={15} />,
    titleKey: "settings.analytics",
    descKey: "settings.analyticsDesc",
  },
];

// ─── Секция: Аналитика (стоп-слова) ──────────────────────────────────────────

function AnalyticsSection({
  onSaved,
  onError,
}: {
  onSaved: () => void;
  onError: () => void;
}) {
  const [value, setValue] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    api
      .stopWords()
      .then((res) => setValue(res.stopWords ?? ""))
      .catch(() => undefined)
      .finally(() => setLoading(false));
  }, []);

  async function handleSave() {
    setSaving(true);
    try {
      await api.updateStopWords(value);
      onSaved();
    } catch {
      onError();
    }
    setSaving(false);
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--sp-4)" }}>
      <FieldRow
        label="Стоп-слова"
        hint="Список слов, исключаемых из «Топ слов» на странице Аналитика. По одному на строку (также допустимы запятые/пробелы). Если оставить пустым — используется встроенный ru/en набор."
      >
        <textarea
          style={{
            ...inputStyle,
            minHeight: 180,
            fontFamily: "var(--font-mono, ui-monospace, monospace)",
            fontSize: "var(--text-xs)",
            lineHeight: "var(--leading-normal)",
            resize: "vertical",
          }}
          value={loading ? "загрузка..." : value}
          disabled={loading}
          onChange={(e) => setValue(e.target.value)}
          placeholder={"и\nна\nне\nкак\n..."}
          spellCheck={false}
        />
      </FieldRow>

      <div
        style={{
          display: "flex",
          alignItems: "center",
          paddingTop: "var(--sp-2)",
          borderTop: "1px solid var(--border)",
        }}
      >
        <Button variant="primary" size="sm" loading={saving} onClick={handleSave}>
          Сохранить
        </Button>
      </div>
    </div>
  );
}

// ─── Переиспользуемые стили ──────────────────────────────────────────────────

const inputStyle: React.CSSProperties = {
  marginTop: 2,
  padding: "7px 10px",
  borderRadius: "var(--radius-sm)",
  border: "1px solid var(--border-strong)",
  background: "var(--bg-inset)",
  color: "var(--text)",
  fontSize: "var(--text-base)",
  outline: "none",
  width: "100%",
  boxSizing: "border-box",
};

const groupHeadingStyle: React.CSSProperties = {
  fontSize: "var(--text-xs)",
  fontWeight: 600,
  color: "var(--text-muted)",
  letterSpacing: "var(--tracking-wide)",
  textTransform: "uppercase",
  marginBottom: "var(--sp-3)",
};

// ─── Toast ───────────────────────────────────────────────────────────────────

interface ToastState {
  message: string;
  type: "success" | "error";
}

function Toast({ toast, onDone }: { toast: ToastState; onDone: () => void }) {
  useEffect(() => {
    const id = setTimeout(onDone, 2800);
    return () => clearTimeout(id);
  }, [toast, onDone]);

  const isSuccess = toast.type === "success";
  return (
    <div
      role="status"
      style={{
        position: "fixed",
        bottom: "var(--sp-6)",
        right: "var(--sp-6)",
        zIndex: 200,
        display: "flex",
        alignItems: "center",
        gap: "var(--sp-2)",
        padding: "10px var(--sp-4)",
        borderRadius: "var(--radius-md)",
        background: isSuccess ? "var(--success-soft)" : "var(--error-soft)",
        border: `1px solid ${
          isSuccess ? "rgba(92,214,143,0.28)" : "rgba(255,122,122,0.28)"
        }`,
        color: isSuccess ? "var(--success)" : "var(--error)",
        fontSize: "var(--text-sm)",
        fontWeight: 500,
        boxShadow: "var(--shadow-md)",
        animation: "slide-up var(--duration-normal) var(--ease) both",
      }}
    >
      {isSuccess ? <CheckCircle2 size={14} /> : <AlertTriangle size={14} />}
      {toast.message}
    </div>
  );
}

// ─── Поле формы ──────────────────────────────────────────────────────────────

function FieldRow({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
      <span
        style={{
          fontSize: "var(--text-sm)",
          fontWeight: 600,
          color: "var(--text-secondary)",
        }}
      >
        {label}
      </span>
      {hint && (
        <span
          style={{
            fontSize: "var(--text-xs)",
            color: "var(--text-muted)",
            lineHeight: "var(--leading-normal)",
          }}
        >
          {hint}
        </span>
      )}
      {children}
    </div>
  );
}

// ─── Секция: Транскрипция ────────────────────────────────────────────────────

interface TranscriptionSectionProps {
  settings: SettingsResponse;
  onSaved: () => void;
  onError: () => void;
}

function TranscriptionSection({
  settings,
  onSaved,
  onError,
}: TranscriptionSectionProps) {
  const { t } = useTranslation();
  const [saving, setSaving] = useState(false);
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

  const isGPU = form.device === "cuda";
  const computeOptions = isGPU
    ? (settings.options.cuda ?? ["float16"])
    : (settings.options.cpu ?? ["int8", "float32"]);

  async function handleSave() {
    setSaving(true);
    try {
      await api.updateSettings({ ...form, uiTheme });
      onSaved();
    } catch {
      onError();
    }
    setSaving(false);
  }

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        gap: "var(--sp-6)",
      }}
    >
      {/* ── Группа: Движок ── */}
      <div>
        <p style={groupHeadingStyle}>{t("settings.groupEngine")}</p>
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(auto-fill, minmax(220px, 1fr))",
            gap: "var(--sp-4)",
          }}
        >
          <FieldRow label={t("settings.backend")} hint={t("settings.backendHint")}>
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
          </FieldRow>

          <FieldRow label={t("settings.model")} hint={t("settings.modelHint")}>
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
          </FieldRow>
        </div>
      </div>

      {/* ── Группа: Железо ── */}
      <div>
        <p style={groupHeadingStyle}>{t("settings.groupHardware")}</p>
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(auto-fill, minmax(220px, 1fr))",
            gap: "var(--sp-4)",
          }}
        >
          <FieldRow label={t("settings.device")} hint={t("settings.deviceHint")}>
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
          </FieldRow>

          <FieldRow
            label={t("settings.computeType")}
            hint={
              isGPU
                ? t("settings.computeTypeHintGPU")
                : t("settings.computeTypeHintCPU")
            }
          >
            <select
              style={inputStyle}
              value={form.computeType}
              onChange={(e) =>
                setForm({ ...form, computeType: e.target.value })
              }
            >
              {computeOptions.map((c) => (
                <option key={c} value={c}>
                  {c}
                </option>
              ))}
            </select>
          </FieldRow>
        </div>
      </div>

      {/* ── Группа: Язык и качество ── */}
      <div>
        <p style={groupHeadingStyle}>{t("settings.groupQuality")}</p>
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(auto-fill, minmax(220px, 1fr))",
            gap: "var(--sp-4)",
          }}
        >
          <FieldRow
            label={t("settings.language")}
            hint={t("settings.languageHint")}
          >
            <input
              style={inputStyle}
              value={form.language}
              onChange={(e) => setForm({ ...form, language: e.target.value })}
              placeholder={t("settings.auto")}
              spellCheck={false}
            />
          </FieldRow>

          <FieldRow
            label={t("settings.beamSize")}
            hint={t("settings.beamSizeHint")}
          >
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
          </FieldRow>
        </div>

        {/* ── Переключатель VAD ── */}
        <label
          style={{
            marginTop: "var(--sp-4)",
            display: "flex",
            alignItems: "flex-start",
            gap: "var(--sp-3)",
            padding: "var(--sp-3) var(--sp-4)",
            borderRadius: "var(--radius-sm)",
            border: `1px solid ${
              form.vadEnabled ? "var(--border-accent)" : "var(--border)"
            }`,
            background: form.vadEnabled ? "var(--accent-muted)" : "transparent",
            cursor: "pointer",
            transition:
              "background var(--duration-fast) var(--ease), border-color var(--duration-fast) var(--ease)",
          }}
        >
          <input
            type="checkbox"
            checked={form.vadEnabled}
            onChange={(e) =>
              setForm({ ...form, vadEnabled: e.target.checked })
            }
            style={{
              accentColor: "var(--accent)",
              marginTop: 2,
              flexShrink: 0,
              cursor: "pointer",
            }}
          />
          <div>
            <div
              style={{
                fontSize: "var(--text-sm)",
                fontWeight: 600,
                color: "var(--text-secondary)",
              }}
            >
              {t("settings.vadFilter")}
            </div>
            <div
              style={{
                fontSize: "var(--text-xs)",
                color: "var(--text-muted)",
                marginTop: 2,
                lineHeight: "var(--leading-normal)",
              }}
            >
              {t("settings.vadHint")}
            </div>
          </div>
        </label>
      </div>

      {/* ── Предупреждения от бэкенда ── */}
      {settings.warnings.length > 0 && (
        <div
          style={{
            display: "flex",
            gap: "var(--sp-2)",
            padding: "var(--sp-3) var(--sp-4)",
            borderRadius: "var(--radius-sm)",
            background: "var(--warning-soft)",
            border: "1px solid rgba(255,186,73,0.2)",
            color: "var(--warning)",
            fontSize: "var(--text-sm)",
            lineHeight: "var(--leading-normal)",
          }}
        >
          <AlertTriangle
            size={14}
            style={{ flexShrink: 0, marginTop: 2 }}
          />
          <span>{settings.warnings.join(" · ")}</span>
        </div>
      )}

      {/* ── Действия ── */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          paddingTop: "var(--sp-2)",
          borderTop: "1px solid var(--border)",
        }}
      >
        <Button
          variant="primary"
          size="sm"
          loading={saving}
          onClick={handleSave}
        >
          {t("settings.save")}
        </Button>
      </div>
    </div>
  );
}

// ─── Sidebar ─────────────────────────────────────────────────────────────────

function Sidebar({
  active,
  onChange,
}: {
  active: Section;
  onChange: (s: Section) => void;
}) {
  const { t } = useTranslation();
  return (
    <nav
      aria-label="Settings sections"
      style={{
        background: "var(--bg-card)",
        border: "1px solid var(--border)",
        borderRadius: "var(--radius-md)",
        padding: "var(--sp-2)",
        display: "flex",
        flexDirection: "column",
        gap: 2,
        alignSelf: "flex-start",
        position: "sticky",
        top: "var(--sp-4)",
      }}
    >
      {NAV_ITEMS.map((item) => {
        const isActive = item.id === active;
        return (
          <button
            key={item.id}
            type="button"
            onClick={() => onChange(item.id)}
            style={{
              display: "flex",
              alignItems: "center",
              gap: "var(--sp-2)",
              padding: "8px 10px",
              borderRadius: "var(--radius-sm)",
              border: "1px solid transparent",
              background: isActive ? "var(--accent-soft)" : "transparent",
              color: isActive ? "var(--accent)" : "var(--text-secondary)",
              fontSize: "var(--text-sm)",
              fontWeight: isActive ? 600 : 500,
              textAlign: "left",
              cursor: "pointer",
              transition:
                "background var(--duration-fast) var(--ease), color var(--duration-fast) var(--ease)",
            }}
            onMouseEnter={(e) => {
              if (!isActive) {
                e.currentTarget.style.background = "var(--bg-card-hover)";
                e.currentTarget.style.color = "var(--text)";
              }
            }}
            onMouseLeave={(e) => {
              if (!isActive) {
                e.currentTarget.style.background = "transparent";
                e.currentTarget.style.color = "var(--text-secondary)";
              }
            }}
          >
            {item.icon}
            {t(item.titleKey)}
          </button>
        );
      })}
    </nav>
  );
}

// ─── Главный компонент ───────────────────────────────────────────────────────

export function SettingsPage() {
  const { t } = useTranslation();
  const [settings, setSettings] = useState<SettingsResponse | null>(null);
  const [loadError, setLoadError] = useState(false);
  const [active, setActive] = useState<Section>("transcription");
  const [toast, setToast] = useState<ToastState | null>(null);

  useEffect(() => {
    api
      .settings()
      .then(setSettings)
      .catch(() => setLoadError(true));
  }, []);

  const activeItem = NAV_ITEMS.find((n) => n.id === active) ?? NAV_ITEMS[0];

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

        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "var(--sp-2)",
          }}
        >
          <SettingsIcon size={16} style={{ color: "var(--text-muted)" }} />
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

      {/* ── Ошибка загрузки ── */}
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

      {/* ── Состояние загрузки ── */}
      {!settings && !loadError && (
        <div
          style={{
            color: "var(--text-muted)",
            fontSize: "var(--text-sm)",
          }}
        >
          {t("settings.loading")}
        </div>
      )}

      {/* ── Тело: sidebar + контент ── */}
      {settings && (
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "minmax(180px, 220px) 1fr",
            gap: "var(--sp-6)",
            alignItems: "start",
          }}
        >
          <Sidebar active={active} onChange={setActive} />

          <section
            style={{
              background: "var(--bg-card)",
              border: "1px solid var(--border)",
              borderRadius: "var(--radius-md)",
              padding: "var(--sp-6)",
              minWidth: 0,
            }}
          >
            {/* Заголовок секции */}
            <header
              style={{
                marginBottom: "var(--sp-5)",
                paddingBottom: "var(--sp-4)",
                borderBottom: "1px solid var(--border)",
              }}
            >
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: "var(--sp-2)",
                  color: "var(--accent)",
                  marginBottom: "var(--sp-1)",
                }}
              >
                {activeItem.icon}
                <h2
                  style={{
                    fontSize: "var(--text-lg)",
                    fontWeight: 700,
                    color: "var(--text)",
                    margin: 0,
                  }}
                >
                  {t(activeItem.titleKey)}
                </h2>
              </div>
              <p
                style={{
                  fontSize: "var(--text-sm)",
                  color: "var(--text-muted)",
                  lineHeight: "var(--leading-normal)",
                  margin: 0,
                }}
              >
                {t(activeItem.descKey)}
              </p>
            </header>

            {/* Тело секции */}
            {active === "transcription" && (
              <TranscriptionSection
                settings={settings}
                onSaved={() =>
                  setToast({ message: t("settings.saved"), type: "success" })
                }
                onError={() =>
                  setToast({
                    message: t("settings.saveError"),
                    type: "error",
                  })
                }
              />
            )}
            {active === "rules" && <TriggerRules />}
            {active === "analytics" && (
              <AnalyticsSection
                onSaved={() =>
                  setToast({ message: t("settings.saved"), type: "success" })
                }
                onError={() =>
                  setToast({
                    message: t("settings.saveError"),
                    type: "error",
                  })
                }
              />
            )}
          </section>
        </div>
      )}

      {/* ── Toast ── */}
      {toast && <Toast toast={toast} onDone={() => setToast(null)} />}
    </div>
  );
}
