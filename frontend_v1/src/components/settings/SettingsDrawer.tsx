import { useEffect, useState } from "react";
import { X } from "lucide-react";
import { api } from "../../api/client";
import type { SettingsResponse } from "../../models/types";
import { Button } from "../ui/Button";
import { TriggerRules } from "./TriggerRules";

interface SettingsDrawerProps {
  open: boolean;
  onClose: () => void;
}

export function SettingsDrawer({ open, onClose }: SettingsDrawerProps) {
  const [settings, setSettings] = useState<SettingsResponse | null>(null);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({
    backend: "faster-whisper",
    modelName: "base",
    device: "cpu",
    computeType: "int8",
    language: "",
    beamSize: 5,
    vadEnabled: true,
    uiTheme: "new",
  });

  useEffect(() => {
    if (!open) return;
    api.settings().then((s) => {
      setSettings(s);
      setForm({
        backend: s.profile.backend || "faster-whisper",
        modelName: s.profile.modelName || "base",
        device: s.profile.device || "cpu",
        computeType: s.profile.computeType || "int8",
        language: s.profile.language || "",
        beamSize: s.profile.beamSize || 5,
        vadEnabled: s.profile.vadEnabled ?? true,
        uiTheme: s.profile.uiTheme || "new",
      });
    }).catch(() => {});
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") onClose(); };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  async function handleSave() {
    setSaving(true);
    try {
      await api.updateSettings(form);
    } catch {}
    setSaving(false);
  }

  const computeOptions = form.device === "cuda"
    ? (settings?.options.cuda ?? ["float16"])
    : (settings?.options.cpu ?? ["int8", "float32"]);

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

  return (
    <>
      {/* Overlay */}
      {open && (
        <div
          onClick={onClose}
          style={{
            position: "fixed",
            inset: 0,
            background: "var(--bg-overlay)",
            zIndex: "var(--z-drawer-overlay)" as unknown as number,
            animation: "fade-in var(--duration-fast) var(--ease)",
          }}
        />
      )}

      {/* Drawer */}
      <aside
        style={{
          position: "fixed",
          top: 0,
          right: 0,
          width: 400,
          maxWidth: "100vw",
          height: "100vh",
          background: "var(--bg-surface)",
          borderLeft: "1px solid var(--border)",
          zIndex: "var(--z-drawer)" as unknown as number,
          transform: open ? "translateX(0)" : "translateX(100%)",
          transition: `transform var(--duration-normal) var(--ease)`,
          display: "flex",
          flexDirection: "column",
          overflow: "hidden",
        }}
      >
        {/* Header */}
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            padding: "var(--sp-4) var(--sp-5)",
            borderBottom: "1px solid var(--border)",
          }}
        >
          <h2 style={{ fontSize: "var(--text-md)", fontWeight: 700 }}>Settings</h2>
          <button
            onClick={onClose}
            aria-label="Close"
            style={{
              width: 28,
              height: 28,
              borderRadius: "var(--radius-xs)",
              display: "grid",
              placeItems: "center",
              color: "var(--text-muted)",
            }}
          >
            <X size={16} />
          </button>
        </div>

        {/* Content */}
        <div style={{ flex: 1, overflow: "auto", padding: "var(--sp-5)" }}>
          {settings && (
            <div style={{ display: "flex", flexDirection: "column", gap: "var(--sp-5)" }}>
              <div>
                <h3 style={{ fontSize: "var(--text-sm)", fontWeight: 600, color: "var(--text-secondary)", letterSpacing: "var(--tracking-wide)", textTransform: "uppercase", marginBottom: "var(--sp-3)" }}>
                  Transcription
                </h3>

                <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "var(--sp-3)" }}>
                  <label style={labelStyle}>
                    Backend
                    <select style={inputStyle} value={form.backend} onChange={(e) => setForm({ ...form, backend: e.target.value })}>
                      {settings.options.backends.map((b) => <option key={b} value={b}>{b}</option>)}
                    </select>
                  </label>
                  <label style={labelStyle}>
                    Model
                    <select style={inputStyle} value={form.modelName} onChange={(e) => setForm({ ...form, modelName: e.target.value })}>
                      {settings.options.models.map((m) => <option key={m} value={m}>{m}</option>)}
                    </select>
                  </label>
                  <label style={labelStyle}>
                    Device
                    <select style={inputStyle} value={form.device} onChange={(e) => setForm({ ...form, device: e.target.value, computeType: e.target.value === "cuda" ? "float16" : "int8" })}>
                      {settings.options.devices.map((d) => <option key={d} value={d}>{d}</option>)}
                    </select>
                  </label>
                  <label style={labelStyle}>
                    Compute type
                    <select style={inputStyle} value={form.computeType} onChange={(e) => setForm({ ...form, computeType: e.target.value })}>
                      {computeOptions.map((c) => <option key={c} value={c}>{c}</option>)}
                    </select>
                  </label>
                  <label style={labelStyle}>
                    Language
                    <input style={inputStyle} value={form.language} onChange={(e) => setForm({ ...form, language: e.target.value })} placeholder="auto" />
                  </label>
                  <label style={labelStyle}>
                    Beam size
                    <input style={inputStyle} type="number" min={1} max={10} value={form.beamSize} onChange={(e) => setForm({ ...form, beamSize: Number(e.target.value) })} />
                  </label>
                </div>

                <label style={{ ...labelStyle, flexDirection: "row", alignItems: "center", gap: 8, marginTop: "var(--sp-3)" }}>
                  <input type="checkbox" checked={form.vadEnabled} onChange={(e) => setForm({ ...form, vadEnabled: e.target.checked })} style={{ accentColor: "var(--accent)" }} />
                  VAD filter
                </label>

                {settings.warnings.length > 0 && (
                  <div style={{ marginTop: "var(--sp-3)", padding: "var(--sp-2) var(--sp-3)", borderRadius: "var(--radius-sm)", background: "var(--warning-soft)", color: "var(--warning)", fontSize: "var(--text-sm)" }}>
                    {settings.warnings.join(". ")}
                  </div>
                )}

                <div style={{ marginTop: "var(--sp-4)" }}>
                  <Button variant="primary" size="sm" loading={saving} onClick={handleSave}>
                    Save settings
                  </Button>
                </div>
              </div>

              <div style={{ borderTop: "1px solid var(--border)", paddingTop: "var(--sp-5)" }}>
                <TriggerRules />
              </div>
            </div>
          )}
        </div>
      </aside>
    </>
  );
}
