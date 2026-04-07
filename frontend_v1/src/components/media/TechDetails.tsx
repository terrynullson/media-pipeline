import { AlertCircle } from "lucide-react";
import type { MediaDetailResponse } from "../../models/types";
import { StatusChip } from "../ui/StatusChip";
import { Accordion } from "../ui/Accordion";

interface TechDetailsProps {
  pipeline: MediaDetailResponse["pipeline"];
  settingsSnapshot: MediaDetailResponse["settingsSnapshot"];
}

const kvRowStyle: React.CSSProperties = {
  display: "flex",
  justifyContent: "space-between",
  alignItems: "baseline",
  padding: "8px 0",
  borderBottom: "1px solid var(--border)",
  gap: "var(--sp-3)",
};

const labelStyle: React.CSSProperties = {
  color: "var(--text-muted)",
  fontSize: "var(--text-sm)",
  flexShrink: 0,
};

const valueStyle: React.CSSProperties = {
  color: "var(--text)",
  fontSize: "var(--text-sm)",
  fontWeight: 500,
  textAlign: "right" as const,
  wordBreak: "break-word" as const,
};

function SectionTitle({ children }: { children: React.ReactNode }) {
  return (
    <h4
      style={{
        fontSize: "var(--text-xs)",
        fontWeight: 600,
        color: "var(--text-muted)",
        textTransform: "uppercase",
        letterSpacing: "var(--tracking-wide)",
        marginTop: "var(--sp-4)",
        marginBottom: "var(--sp-1)",
      }}
    >
      {children}
    </h4>
  );
}

export function TechDetails({ pipeline, settingsSnapshot }: TechDetailsProps) {
  return (
    <Accordion title="Technical Details">
      {/* Pipeline steps */}
      <SectionTitle>Pipeline Steps</SectionTitle>
      {pipeline.steps.map((step) => (
        <div
          key={step.label}
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: "var(--sp-3)",
            padding: "6px 0",
            borderBottom: "1px solid var(--border)",
            flexWrap: "wrap",
          }}
        >
          <span style={{ color: "var(--text)", fontSize: "var(--text-sm)", fontWeight: 500 }}>
            {step.label}
          </span>
          <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)" }}>
            <StatusChip label={step.statusLabel} tone={step.tone} />
            <span style={{ color: "var(--text-muted)", fontSize: "var(--text-xs)" }}>
              {step.timingText}
            </span>
            {step.durationLabel && (
              <span style={{ color: "var(--text-secondary)", fontSize: "var(--text-xs)" }}>
                {step.durationLabel}
              </span>
            )}
          </div>
        </div>
      ))}

      {/* Settings snapshot */}
      {!settingsSnapshot.settingsUnavailable && settingsSnapshot.settings.length > 0 && (
        <>
          <SectionTitle>Settings Snapshot</SectionTitle>
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "1fr 1fr",
              gap: "0 var(--sp-5)",
            }}
          >
            {settingsSnapshot.settings.map((kv) => (
              <div key={kv.label} style={kvRowStyle}>
                <span style={labelStyle}>{kv.label}</span>
                <span style={valueStyle}>{kv.value}</span>
              </div>
            ))}
          </div>
        </>
      )}

      {/* Runtime snapshot */}
      {settingsSnapshot.runtimeSnapshot.length > 0 && (
        <>
          <SectionTitle>Runtime Snapshot</SectionTitle>
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "1fr 1fr",
              gap: "0 var(--sp-5)",
            }}
          >
            {settingsSnapshot.runtimeSnapshot.map((kv) => (
              <div key={kv.label} style={kvRowStyle}>
                <span style={labelStyle}>{kv.label}</span>
                <span style={valueStyle}>{kv.value}</span>
              </div>
            ))}
          </div>
        </>
      )}

      {/* Runtime policy */}
      {settingsSnapshot.runtimePolicy?.visible && (
        <>
          <SectionTitle>{settingsSnapshot.runtimePolicy.title || "Runtime Policy"}</SectionTitle>
          <p
            style={{
              color: "var(--text-secondary)",
              fontSize: "var(--text-sm)",
              lineHeight: "var(--leading-normal)",
              margin: "var(--sp-1) 0",
            }}
          >
            {settingsSnapshot.runtimePolicy.summary}
          </p>
          {settingsSnapshot.runtimePolicy.warnings &&
            settingsSnapshot.runtimePolicy.warnings.length > 0 && (
              <div style={{ marginTop: "var(--sp-2)" }}>
                {settingsSnapshot.runtimePolicy.warnings.map((w, i) => (
                  <div
                    key={i}
                    style={{
                      display: "flex",
                      alignItems: "flex-start",
                      gap: "var(--sp-2)",
                      color: "var(--warning)",
                      fontSize: "var(--text-xs)",
                      marginBottom: "var(--sp-1)",
                    }}
                  >
                    <AlertCircle size={12} style={{ flexShrink: 0, marginTop: 2 }} />
                    <span>{w}</span>
                  </div>
                ))}
              </div>
            )}
        </>
      )}

      {/* Settings warnings */}
      {settingsSnapshot.settingsWarnings.length > 0 && (
        <>
          <SectionTitle>Warnings</SectionTitle>
          {settingsSnapshot.settingsWarnings.map((w, i) => (
            <div
              key={i}
              style={{
                display: "flex",
                alignItems: "flex-start",
                gap: "var(--sp-2)",
                color: "var(--warning)",
                fontSize: "var(--text-sm)",
                marginBottom: "var(--sp-1)",
              }}
            >
              <AlertCircle size={13} style={{ flexShrink: 0, marginTop: 2 }} />
              <span>{w}</span>
            </div>
          ))}
        </>
      )}
    </Accordion>
  );
}
