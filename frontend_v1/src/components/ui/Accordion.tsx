import { useState, type ReactNode } from "react";
import { ChevronDown } from "lucide-react";

interface AccordionProps {
  title: string;
  defaultOpen?: boolean;
  children: ReactNode;
}

export function Accordion({ title, defaultOpen = false, children }: AccordionProps) {
  const [open, setOpen] = useState(defaultOpen);

  return (
    <div
      style={{
        border: "1px solid var(--border)",
        borderRadius: "var(--radius-md)",
        overflow: "hidden",
      }}
    >
      <button
        onClick={() => setOpen(!open)}
        aria-expanded={open}
        style={{
          width: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: "var(--sp-3)",
          padding: "var(--sp-3) var(--sp-4)",
          background: "var(--bg-surface)",
          color: "var(--text-secondary)",
          fontSize: "var(--text-sm)",
          fontWeight: 600,
          letterSpacing: "var(--tracking-wide)",
          textTransform: "uppercase",
        }}
      >
        {title}
        <ChevronDown
          size={14}
          style={{
            transition: "transform var(--duration-fast) var(--ease)",
            transform: open ? "rotate(180deg)" : "rotate(0deg)",
            color: "var(--text-muted)",
          }}
        />
      </button>
      <div
        style={{
          display: open ? "block" : "none",
          padding: "var(--sp-4)",
          background: "var(--bg-card)",
        }}
      >
        {children}
      </div>
    </div>
  );
}
