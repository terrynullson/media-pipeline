import { FormEvent, useEffect, useMemo, useState } from "react";
import { ArrowRightLeft, Save, SlidersHorizontal } from "lucide-react";
import { api } from "../../api/client";
import type { SettingsResponse, TriggerRule } from "../../models/types";
import { EmptyState, SectionCard, StatusBadge } from "../common/ui";

const fallbackSettings: SettingsResponse = {
  profile: {
    backend: "faster-whisper",
    modelName: "tiny",
    device: "cpu",
    computeType: "int8",
    language: "",
    beamSize: 5,
    vadEnabled: true,
    uiTheme: "new"
  },
  warnings: [],
  ui: {
    theme: "new",
    legacyAppURL: "/app",
    modernAppURL: "/app-v1",
    preferredAppURL: "/app-v1",
    workspaceURL: "/workspace"
  },
  options: {
    backends: ["faster-whisper"],
    models: ["tiny", "base", "small"],
    devices: ["cpu", "cuda"],
    cpu: ["int8", "float32"],
    cuda: ["float16", "int8_float16", "int8_float32"],
    themes: ["old", "new"]
  }
};

export function Settings() {
  const [settings, setSettings] = useState<SettingsResponse>(fallbackSettings);
  const [rules, setRules] = useState<TriggerRule[]>([]);
  const [flash, setFlash] = useState("");

  useEffect(() => {
    api.settings().then(setSettings).catch(() => setSettings(fallbackSettings));
    api.rules().then(setRules).catch(() => setRules([]));
  }, []);

  const computeTypes = useMemo(
    () => (settings.profile.device === "cuda" ? settings.options.cuda : settings.options.cpu),
    [settings.options.cpu, settings.options.cuda, settings.profile.device]
  );

  async function saveSettings(event: FormEvent) {
    event.preventDefault();
    try {
      await api.updateSettings(settings.profile);
      setFlash("Настройки сохранены.");
    } catch {
      setFlash("Не удалось сохранить настройки.");
    }
  }

  async function changeTheme(nextTheme: "old" | "new") {
    try {
      const result = await api.updateUITheme(nextTheme);
      setSettings((current) => ({
        ...current,
        profile: { ...current.profile, uiTheme: result.uiTheme },
        ui: { ...current.ui, theme: result.uiTheme, preferredAppURL: result.preferredAppURL }
      }));
      setFlash(`Предпочтение интерфейса сохранено. Основной маршрут теперь ведёт на ${result.uiTheme === "new" ? "новый" : "старый"} UI.`);
    } catch {
      setFlash("Не удалось переключить предпочтение интерфейса.");
    }
  }

  async function addRule(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formData = new FormData(event.currentTarget);
    try {
      await api.createRule({
        name: String(formData.get("name") || ""),
        category: String(formData.get("category") || ""),
        pattern: String(formData.get("pattern") || ""),
        matchMode: String(formData.get("matchMode") || "contains")
      });
      setRules(await api.rules());
      event.currentTarget.reset();
      setFlash("Правило добавлено.");
    } catch {
      setFlash("Не удалось добавить правило.");
    }
  }

  async function toggleRule(rule: TriggerRule) {
    await api.toggleRule(rule.id, !rule.enabled);
    setRules(await api.rules());
  }

  async function removeRule(ruleId: number) {
    await api.deleteRule(ruleId);
    setRules(await api.rules());
  }

  return (
    <div className="settings-layout">
      <aside className="settings-side">
        <div className="settings-side-label">SETTINGS</div>
        <button type="button" className="settings-side-item active">
          <SlidersHorizontal size={15} />
          <span>Транскрипция</span>
        </button>
        <button type="button" className="settings-side-item">
          <ArrowRightLeft size={15} />
          <span>Интерфейс</span>
        </button>
      </aside>

      <div className="settings-content">
        {flash ? <div className="flash-banner inline">{flash}</div> : null}

        <SectionCard title="Предпочтение интерфейса" subtitle="Старый и новый UI живут параллельно">
          <div className="theme-toggle-row">
            <button type="button" className={`theme-choice${settings.profile.uiTheme === "old" ? " active" : ""}`} onClick={() => void changeTheme("old")}>
              <span>Старый интерфейс</span>
              <small>Маршрут `/app`, текущий legacy UI остаётся без изменений</small>
            </button>
            <button type="button" className={`theme-choice${settings.profile.uiTheme === "new" ? " active" : ""}`} onClick={() => void changeTheme("new")}>
              <span>Новый интерфейс</span>
              <small>Маршрут `/app-v1`, новый shell в стиле `_example_src_v1`</small>
            </button>
          </div>
          <div className="inline-note">
            Основной переключаемый маршрут: <strong>{settings.ui.workspaceURL}</strong>. Сейчас он ведёт на <strong>{settings.ui.preferredAppURL}</strong>.
          </div>
        </SectionCard>

        <div className="detail-grid">
          <SectionCard title="Transcription settings" subtitle="Только реальные backend-настройки">
            <form className="settings-form" onSubmit={saveSettings}>
              <label>
                <span>Backend</span>
                <select value={settings.profile.backend} onChange={(event) => setSettings((current) => ({ ...current, profile: { ...current.profile, backend: event.target.value } }))}>
                  {settings.options.backends.map((item) => <option key={item}>{item}</option>)}
                </select>
              </label>
              <label>
                <span>Model</span>
                <select value={settings.profile.modelName} onChange={(event) => setSettings((current) => ({ ...current, profile: { ...current.profile, modelName: event.target.value } }))}>
                  {settings.options.models.map((item) => <option key={item}>{item}</option>)}
                </select>
              </label>
              <label>
                <span>Device</span>
                <select value={settings.profile.device} onChange={(event) => setSettings((current) => ({ ...current, profile: { ...current.profile, device: event.target.value } }))}>
                  {settings.options.devices.map((item) => <option key={item}>{item}</option>)}
                </select>
              </label>
              <label>
                <span>Compute type</span>
                <select value={settings.profile.computeType} onChange={(event) => setSettings((current) => ({ ...current, profile: { ...current.profile, computeType: event.target.value } }))}>
                  {computeTypes.map((item) => <option key={item}>{item}</option>)}
                </select>
              </label>
              <label>
                <span>Language</span>
                <input value={settings.profile.language} onChange={(event) => setSettings((current) => ({ ...current, profile: { ...current.profile, language: event.target.value } }))} placeholder="auto" />
              </label>
              <label>
                <span>Beam size</span>
                <input type="number" min={1} max={10} value={settings.profile.beamSize} onChange={(event) => setSettings((current) => ({ ...current, profile: { ...current.profile, beamSize: Number(event.target.value) } }))} />
              </label>
              <label className="check-field">
                <input type="checkbox" checked={settings.profile.vadEnabled} onChange={(event) => setSettings((current) => ({ ...current, profile: { ...current.profile, vadEnabled: event.target.checked } }))} />
                <span>VAD enabled</span>
              </label>
              <button type="submit" className="primary-action inline-action">
                <Save size={15} />
                <span>Сохранить</span>
              </button>
            </form>

            {settings.warnings.length > 0 ? (
              <div className="warning-list">
                {settings.warnings.map((warning) => <div key={warning} className="empty-panel compact">{warning}</div>)}
              </div>
            ) : null}
          </SectionCard>

          <SectionCard title="Trigger rules" subtitle="Реальные сущности продукта">
            <form className="settings-form compact-form" onSubmit={addRule}>
              <label>
                <span>Название</span>
                <input name="name" required />
              </label>
              <label>
                <span>Категория</span>
                <input name="category" />
              </label>
              <label>
                <span>Паттерн</span>
                <input name="pattern" required />
              </label>
              <label>
                <span>Режим</span>
                <select name="matchMode" defaultValue="contains">
                  <option value="contains">contains</option>
                  <option value="exact">exact</option>
                  <option value="regex">regex</option>
                </select>
              </label>
              <button type="submit" className="primary-action inline-action">Добавить правило</button>
            </form>

            {rules.length === 0 ? (
              <EmptyState text="Правил пока нет." />
            ) : (
              <div className="table-list">
                {rules.map((rule) => (
                  <div key={rule.id} className="rule-item">
                    <div className="table-item-main">
                      <div className="table-item-title">{rule.name}</div>
                      <div className="table-item-subtitle">{rule.pattern} • {rule.matchMode}</div>
                    </div>
                    <div className="rule-actions">
                      <StatusBadge label={rule.enabledLabel} tone={rule.enabledTone} />
                      <button type="button" className="secondary-button" onClick={() => void toggleRule(rule)}>
                        {rule.toggleLabel}
                      </button>
                      <button type="button" className="secondary-button danger" onClick={() => void removeRule(rule.id)}>
                        Удалить
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </SectionCard>
        </div>
      </div>
    </div>
  );
}
