import { FormEvent, useEffect, useMemo, useState } from "react";
import { api } from "../lib/api";
import { mockRules, mockSettings } from "../lib/mocks";
import type { SettingsResponse, TriggerRule } from "../lib/types";
import { Card, EmptyState, SectionHeader, StatusBadge } from "../shared/ui";

export function SettingsPage() {
  const [settings, setSettings] = useState<SettingsResponse>(mockSettings);
  const [rules, setRules] = useState<TriggerRule[]>(mockRules);
  const [status, setStatus] = useState("");
  const computeOptions = useMemo(() => (settings.profile.device === "cuda" ? settings.options.cuda : settings.options.cpu), [settings]);

  useEffect(() => {
    api.settings().then(setSettings).catch(() => setSettings(mockSettings));
    api.rules().then(setRules).catch(() => setRules(mockRules));
  }, []);

  async function saveSettings(event: FormEvent) {
    event.preventDefault();
    try {
      await api.updateSettings(settings.profile);
      setStatus("Настройки сохранены.");
    } catch {
      setStatus("Не удалось сохранить настройки.");
    }
  }

  async function addRule(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formData = new FormData(event.currentTarget);
    const payload = {
      name: String(formData.get("name") || ""),
      category: String(formData.get("category") || ""),
      pattern: String(formData.get("pattern") || ""),
      matchMode: String(formData.get("matchMode") || "contains")
    };

    try {
      await api.createRule(payload);
      setStatus("Правило создано.");
      setRules(await api.rules());
      event.currentTarget.reset();
    } catch {
      setStatus("Не удалось создать правило.");
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
    <div className="page-stack">
      <SectionHeader eyebrow="Control" title="Settings" description="Страница настроек собрана в том же panel-layout, но наполнена реальными сущностями media-pipeline." />
      {status ? <div className="flash-banner inline">{status}</div> : null}

      <div className="detail-grid">
        <Card title="Transcription settings" subtitle="Backend / model / runtime">
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
                {computeOptions.map((item) => <option key={item}>{item}</option>)}
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
            <label className="checkbox-row">
              <input type="checkbox" checked={settings.profile.vadEnabled} onChange={(event) => setSettings((current) => ({ ...current, profile: { ...current.profile, vadEnabled: event.target.checked } }))} />
              <span>VAD enabled</span>
            </label>
            <button type="submit" className="primary-button">Save settings</button>
          </form>
          {settings.warnings.length > 0 ? (
            <div className="warning-stack">
              {settings.warnings.map((item) => <div key={item} className="empty-state compact">{item}</div>)}
            </div>
          ) : null}
        </Card>

        <Card title="Trigger rules" subtitle="Existing backend entity">
          <form className="settings-form tight" onSubmit={addRule}>
            <label>
              <span>Name</span>
              <input name="name" required />
            </label>
            <label>
              <span>Category</span>
              <input name="category" />
            </label>
            <label>
              <span>Pattern</span>
              <input name="pattern" required />
            </label>
            <label>
              <span>Match mode</span>
              <select name="matchMode" defaultValue="contains">
                <option value="contains">contains</option>
                <option value="exact">exact</option>
                <option value="regex">regex</option>
              </select>
            </label>
            <button type="submit" className="primary-button">Add rule</button>
          </form>

          {rules.length === 0 ? (
            <EmptyState text="Правил пока нет." />
          ) : (
            <div className="list-stack">
              {rules.map((rule) => (
                <div key={rule.id} className="rule-row">
                  <div>
                    <div className="table-primary">{rule.name}</div>
                    <div className="table-secondary">{rule.pattern} · {rule.matchMode}</div>
                  </div>
                  <div className="rule-actions">
                    <StatusBadge label={rule.enabledLabel} tone={rule.enabledTone} />
                    <button className="ghost-button" type="button" onClick={() => toggleRule(rule)}>{rule.toggleLabel}</button>
                    <button className="ghost-button danger" type="button" onClick={() => removeRule(rule.id)}>Delete</button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </Card>
      </div>
    </div>
  );
}
