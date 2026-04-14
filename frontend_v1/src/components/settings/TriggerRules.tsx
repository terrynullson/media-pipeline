import { useEffect, useState } from "react";
import { Trash2 } from "lucide-react";
import { api } from "../../api/client";
import type { TriggerRule } from "../../models/types";
import { useTranslation } from "../../i18n";
import { Button } from "../ui/Button";
import { StatusChip } from "../ui/StatusChip";

type PreviewResult = {
  totalMatches: number;
  mediaMatches: { mediaId: number; matchCount: number; firstMatchAt: number }[];
  limited: boolean;
} | null;

export function TriggerRules() {
  const { t } = useTranslation();
  const [rules, setRules] = useState<TriggerRule[]>([]);
  const [adding, setAdding] = useState(false);
  const [previewing, setPreviewing] = useState(false);
  const [previewResult, setPreviewResult] = useState<PreviewResult>(null);
  const [form, setForm] = useState({ name: "", category: "", pattern: "", matchMode: "contains" });

  const load = () => api.rules().then(setRules).catch(() => {});

  useEffect(() => { load(); }, []);

  async function handleAdd() {
    if (!form.name || !form.pattern) return;
    setAdding(true);
    try {
      await api.createRule(form);
      setForm({ name: "", category: "", pattern: "", matchMode: "contains" });
      setPreviewResult(null);
      await load();
    } catch {}
    setAdding(false);
  }

  async function handlePreview() {
    if (!form.pattern) return;
    setPreviewing(true);
    setPreviewResult(null);
    try {
      const result = await api.previewTriggerRule({ pattern: form.pattern, matchMode: form.matchMode });
      setPreviewResult(result);
    } catch {}
    setPreviewing(false);
  }

  async function handleToggle(rule: TriggerRule) {
    await api.toggleRule(rule.id, !rule.enabled).catch(() => {});
    await load();
  }

  async function handleDelete(rule: TriggerRule) {
    await api.deleteRule(rule.id).catch(() => {});
    await load();
  }

  const inputStyle: React.CSSProperties = {
    padding: "6px 8px",
    borderRadius: "var(--radius-xs)",
    border: "1px solid var(--border-strong)",
    background: "var(--bg-inset)",
    color: "var(--text)",
    fontSize: "var(--text-sm)",
    outline: "none",
  };

  return (
    <div>
      <h3 style={{ fontSize: "var(--text-sm)", fontWeight: 600, color: "var(--text-secondary)", letterSpacing: "var(--tracking-wide)", textTransform: "uppercase", marginBottom: "var(--sp-3)" }}>
        {t("rules.title")}
      </h3>

      {/* Add form */}
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "var(--sp-2)", marginBottom: "var(--sp-3)" }}>
        <input style={inputStyle} placeholder={t("rules.name")} value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
        <input style={inputStyle} placeholder={t("rules.category")} value={form.category} onChange={(e) => setForm({ ...form, category: e.target.value })} />
        <input style={inputStyle} placeholder={t("rules.pattern")} value={form.pattern} onChange={(e) => setForm({ ...form, pattern: e.target.value })} />
        <select style={inputStyle} value={form.matchMode} onChange={(e) => setForm({ ...form, matchMode: e.target.value })}>
          <option value="contains">{t("rules.contains")}</option>
          <option value="exact">{t("rules.exact")}</option>
        </select>
      </div>
      <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)", flexWrap: "wrap" }}>
        <Button variant="primary" size="sm" loading={adding} onClick={handleAdd}>
          {t("rules.add")}
        </Button>
        <Button variant="ghost" size="sm" loading={previewing} onClick={handlePreview} disabled={!form.pattern}>
          {t("rules.preview")}
        </Button>
      </div>

      {/* Preview result */}
      {previewResult && (
        <div
          style={{
            marginTop: "var(--sp-2)",
            padding: "var(--sp-2) var(--sp-3)",
            borderRadius: "var(--radius-sm)",
            background: "var(--bg-inset)",
            border: "1px solid var(--border)",
            fontSize: "var(--text-sm)",
            color: "var(--text-secondary)",
          }}
        >
          {previewResult.totalMatches === 0 ? (
            <span>{t("rules.previewEmpty")}</span>
          ) : (
            <span>
              {t("rules.previewResult")
                .replace("{matches}", String(previewResult.totalMatches))
                .replace("{files}", String(previewResult.mediaMatches.length))}
              {previewResult.limited && ` (показаны первые ${previewResult.mediaMatches.length})`}
            </span>
          )}
        </div>
      )}

      {/* List */}
      <div style={{ display: "flex", flexDirection: "column", gap: "var(--sp-2)", marginTop: "var(--sp-4)" }}>
        {rules.map((rule) => (
          <div
            key={rule.id}
            style={{
              display: "flex",
              alignItems: "center",
              gap: "var(--sp-3)",
              padding: "var(--sp-2) var(--sp-3)",
              borderRadius: "var(--radius-sm)",
              background: "var(--bg-card)",
              border: "1px solid var(--border)",
              fontSize: "var(--text-sm)",
            }}
          >
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontWeight: 600, color: "var(--text)" }}>{rule.name}</div>
              <div style={{ color: "var(--text-muted)", fontSize: "var(--text-xs)" }}>
                {rule.pattern} &middot; {rule.matchMode}
              </div>
            </div>
            <StatusChip label={rule.enabledLabel} tone={rule.enabledTone} />
            <button
              onClick={() => handleToggle(rule)}
              style={{ fontSize: "var(--text-xs)", color: "var(--text-muted)", padding: "2px 6px", borderRadius: "var(--radius-xs)", border: "1px solid var(--border)" }}
            >
              {rule.toggleLabel}
            </button>
            <button
              onClick={() => handleDelete(rule)}
              style={{ color: "var(--error)", opacity: 0.6, padding: 2 }}
              aria-label={t("action.delete")}
            >
              <Trash2 size={13} />
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}
