interface Tab {
  key: string;
  label: string;
}

interface TabsProps {
  tabs: Tab[];
  activeKey: string;
  onChange: (key: string) => void;
}

export function Tabs({ tabs, activeKey, onChange }: TabsProps) {
  return (
    <div
      role="tablist"
      style={{
        display: "flex",
        gap: "var(--sp-1)",
        borderBottom: "1px solid var(--border)",
        marginBottom: "var(--sp-3)",
      }}
    >
      {tabs.map((tab) => {
        const active = tab.key === activeKey;
        return (
          <button
            key={tab.key}
            role="tab"
            aria-selected={active}
            onClick={() => onChange(tab.key)}
            style={{
              padding: "var(--sp-2) var(--sp-3)",
              fontSize: "var(--text-sm)",
              fontWeight: active ? 600 : 400,
              color: active ? "var(--accent)" : "var(--text-muted)",
              borderBottom: active ? "2px solid var(--accent)" : "2px solid transparent",
              marginBottom: "-1px",
              transition: "color var(--duration-fast) var(--ease), border-color var(--duration-fast) var(--ease)",
              background: "none",
            }}
          >
            {tab.label}
          </button>
        );
      })}
    </div>
  );
}
