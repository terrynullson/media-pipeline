import type { ReactNode, ButtonHTMLAttributes } from "react";
import { Loader2 } from "lucide-react";

type Variant = "primary" | "secondary" | "ghost" | "danger";
type Size = "sm" | "md";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
  loading?: boolean;
  icon?: ReactNode;
}

const base: React.CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  justifyContent: "center",
  gap: "var(--sp-2)",
  fontWeight: 600,
  borderRadius: "var(--radius-sm)",
  transition: "background var(--duration-fast) var(--ease), border-color var(--duration-fast) var(--ease), opacity var(--duration-fast) var(--ease)",
  whiteSpace: "nowrap",
  userSelect: "none",
  border: "1px solid transparent",
};

const variants: Record<Variant, React.CSSProperties> = {
  primary: {
    background: "var(--accent)",
    color: "var(--text-on-accent)",
    borderColor: "var(--accent)",
  },
  secondary: {
    background: "transparent",
    color: "var(--text-secondary)",
    borderColor: "var(--border-strong)",
  },
  ghost: {
    background: "transparent",
    color: "var(--text-secondary)",
  },
  danger: {
    background: "var(--error-soft)",
    color: "var(--error)",
    borderColor: "rgba(255,122,122,0.18)",
  },
};

const sizes: Record<Size, React.CSSProperties> = {
  sm: { padding: "5px 10px", fontSize: "var(--text-sm)" },
  md: { padding: "8px 14px", fontSize: "var(--text-base)" },
};

export function Button({
  variant = "secondary",
  size = "md",
  loading = false,
  icon,
  children,
  disabled,
  style,
  ...rest
}: ButtonProps) {
  return (
    <button
      disabled={disabled || loading}
      style={{
        ...base,
        ...variants[variant],
        ...sizes[size],
        opacity: disabled || loading ? 0.55 : 1,
        cursor: disabled || loading ? "not-allowed" : "pointer",
        ...style,
      }}
      {...rest}
    >
      {loading ? <Loader2 size={size === "sm" ? 13 : 15} style={{ animation: "spin 1s linear infinite" }} /> : icon}
      {children}
    </button>
  );
}
