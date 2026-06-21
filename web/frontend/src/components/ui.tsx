import { useState, type ReactNode } from "react";
import { Copy, Check } from "lucide-react";

// Status → pill class mapping.
export function statusPillClass(status: string): string {
  switch (status) {
    case "ACTIVE":
    case "READY":
      return "pill pill-success";
    case "DISABLED":
    case "DEGRADED":
    case "DESTROY_PENDING":
      return "pill pill-warning";
    case "DESTROYED":
    case "REVOKED":
      return "pill pill-danger";
    case "PRE_ACTIVE":
    case "REGISTERED":
      return "pill pill-info";
    default:
      return "pill pill-neutral";
  }
}

export function StatusPill({ status }: { status: string }) {
  return <span className={statusPillClass(status)}>{status}</span>;
}

export function SuiteBadge({ suite }: { suite: string }) {
  return <span className="suite-badge">{suite}</span>;
}

export function MonoReadout({
  value,
  label,
  copyable = false,
}: {
  value: string;
  label?: string;
  copyable?: boolean;
}) {
  const [copied, setCopied] = useState(false);
  const copy = () => {
    navigator.clipboard.writeText(value);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };
  return (
    <div>
      {label && <div className="input-label" style={{ marginBottom: 6 }}>{label}</div>}
      <div className="mono-readout" style={{ position: "relative" }}>
        {value || <span style={{ color: "var(--text-tertiary)" }}>—</span>}
        {copyable && value && (
          <button
            onClick={copy}
            style={{
              position: "absolute",
              top: 8,
              right: 8,
              background: "var(--bg-surface-2)",
              border: "1px solid var(--border-bright)",
              borderRadius: 2,
              padding: 4,
              cursor: "pointer",
              color: "var(--text-secondary)",
              display: "flex",
            }}
            title="Copy"
          >
            {copied ? <Check size={12} /> : <Copy size={12} />}
          </button>
        )}
      </div>
    </div>
  );
}

export function Panel({
  title,
  action,
  children,
  bodyClass,
}: {
  title?: string;
  action?: ReactNode;
  children: ReactNode;
  bodyClass?: string;
}) {
  return (
    <div className="panel">
      {title && (
        <div className="panel-header">
          <span className="panel-title">{title}</span>
          {action}
        </div>
      )}
      <div className={`panel-body ${bodyClass ?? ""}`}>{children}</div>
    </div>
  );
}

export function EmptyState({ icon, message }: { icon: ReactNode; message: string }) {
  return (
    <div className="empty-state">
      {icon}
      <div>{message}</div>
    </div>
  );
}

export function Loading({ label }: { label?: string }) {
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 10, padding: 20, color: "var(--text-tertiary)" }}>
      <div className="spinner" />
      {label && <span style={{ fontSize: 12, fontFamily: '"JetBrains Mono", monospace' }}>{label}</span>}
    </div>
  );
}

export function ErrorState({ message }: { message: string }) {
  return (
    <div
      style={{
        padding: 20,
        color: "var(--danger)",
        fontFamily: '"JetBrains Mono", monospace',
        fontSize: 12,
        border: "1px solid rgba(248,113,113,0.3)",
        background: "var(--danger-dim)",
        borderRadius: 2,
      }}
    >
      {message}
    </div>
  );
}

// Toast system (simple, via custom event).
export function showToast(message: string, type: "success" | "error" | "info" | "warning" = "info") {
  window.dispatchEvent(new CustomEvent("kvlt-toast", { detail: { message, type } }));
}

export function ToastContainer() {
  const [toasts, setToasts] = useState<{ id: number; message: string; type: string }[]>([]);
  useState(() => {
    window.addEventListener("kvlt-toast", (e) => {
      const detail = (e as CustomEvent).detail;
      const id = Date.now() + Math.random();
      setToasts((prev) => [...prev, { id, ...detail }]);
      setTimeout(() => setToasts((prev) => prev.filter((t) => t.id !== id)), 3500);
    });
  });
  return (
    <div className="toast-container">
      {toasts.map((t) => (
        <div key={t.id} className={`toast toast-${t.type === "warning" ? "error" : t.type}`}>
          {t.message}
        </div>
      ))}
    </div>
  );
}

export function Modal({
  title,
  children,
  onClose,
  onConfirm,
  confirmLabel,
  danger,
}: {
  title: string;
  children: ReactNode;
  onClose: () => void;
  onConfirm?: () => void;
  confirmLabel?: string;
  danger?: boolean;
}) {
  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <span className="panel-title">{title}</span>
        </div>
        <div className="modal-body">{children}</div>
        <div className="modal-footer">
          <button className="btn btn-secondary btn-sm" onClick={onClose}>
            Cancel
          </button>
          {onConfirm && (
            <button className={`btn btn-sm ${danger ? "btn-danger" : "btn-primary"}`} onClick={onConfirm}>
              {confirmLabel ?? "Confirm"}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

export function KVList({ items }: { items: [string, ReactNode][] }) {
  return (
    <dl className="kv-list">
      {items.map(([k, v]) => (
        <>
          <dt key={`k-${k}`}>{k}</dt>
          <dd key={`v-${k}`}>{v}</dd>
        </>
      ))}
    </dl>
  );
}
