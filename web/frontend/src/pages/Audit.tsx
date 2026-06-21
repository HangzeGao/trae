import { useState, useEffect } from "react";
import { PageContainer, Panel, EmptyState } from "../components";
import { ScrollText, AlertTriangle } from "lucide-react";

interface AuditEntry {
  timestamp: string;
  event_id: string;
  action: string;
  target_hash: string;
  actor_hash: string;
  request_id: string;
  high_risk: boolean;
}

const HIGH_RISK_ACTIONS = ["CREATE_CRK", "DESTROY_CRK", "ROTATE_CRK", "SCHEDULE_DESTROY", "REVOKE_NODE", "ALLOCATE_NONCE_RANGE"];

// P0: audit log is read from the local WAL file if available.
// Since the WAL is server-side, this page shows a client-side event log
// captured from mutations performed in this session.
export function AuditPage() {
  const [entries, setEntries] = useState<AuditEntry[]>([]);

  useEffect(() => {
    try {
      const raw = localStorage.getItem("kvlt-audit") ?? "[]";
      setEntries(JSON.parse(raw));
    } catch {
      setEntries([]);
    }
  }, []);

  // Listen for new audit events dispatched by mutations.
  useEffect(() => {
    const handler = (e: Event) => {
      const detail = (e as CustomEvent<AuditEntry>).detail;
      const next = [detail, ...entries].slice(0, 100);
      setEntries(next);
      localStorage.setItem("kvlt-audit", JSON.stringify(next));
    };
    window.addEventListener("kvlt-audit-event", handler);
    return () => window.removeEventListener("kvlt-audit-event", handler);
  }, [entries]);

  return (
    <PageContainer>
      <h1 className="section-title">audit</h1>
      <p className="section-subtitle">session event log · high-risk action tracking · wal skeleton</p>

      <Panel title="Event Log">
        {entries.length === 0 ? (
          <EmptyState icon={<ScrollText />} message="no audit events in this session — perform operations to populate" />
        ) : (
          <table className="data-table">
            <thead>
              <tr>
                <th style={{ width: 20 }}></th>
                <th>Timestamp</th>
                <th>Action</th>
                <th>Event ID</th>
                <th>Target</th>
                <th>Actor</th>
                <th>Request ID</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((e, i) => (
                <tr key={i} style={{ cursor: "default" }}>
                  <td>
                    {e.high_risk && (
                      <AlertTriangle size={13} style={{ color: "var(--accent)" }} />
                    )}
                  </td>
                  <td className="mono" style={{ fontSize: 11 }}>{new Date(e.timestamp).toLocaleString()}</td>
                  <td>
                    <span
                      style={{
                        fontFamily: '"JetBrains Mono", monospace',
                        fontSize: 11,
                        color: e.high_risk ? "var(--accent)" : "var(--text-secondary)",
                        fontWeight: e.high_risk ? 600 : 400,
                      }}
                    >
                      {e.action}
                    </span>
                  </td>
                  <td className="mono" style={{ fontSize: 11 }}>{e.event_id.slice(0, 16)}…</td>
                  <td className="mono" style={{ fontSize: 11 }}>{e.target_hash.slice(0, 16)}…</td>
                  <td className="mono" style={{ fontSize: 11 }}>{e.actor_hash.slice(0, 16)}…</td>
                  <td className="mono" style={{ fontSize: 11 }}>{e.request_id.slice(0, 12)}…</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Panel>

      <div style={{ marginTop: 16 }}>
        <Panel title="WAL Status">
          <div style={{ display: "flex", gap: 24, flexWrap: "wrap" }}>
            <div>
              <div className="input-label">Mode</div>
              <div className="mono" style={{ color: "var(--success)" }}>● fail-closed (high-risk)</div>
            </div>
            <div>
              <div className="input-label">Format</div>
              <div className="mono">append-only · fsync · rotation</div>
            </div>
            <div>
              <div className="input-label">High-Risk Actions</div>
              <div className="mono" style={{ fontSize: 11, color: "var(--text-secondary)" }}>
                {HIGH_RISK_ACTIONS.join(" · ")}
              </div>
            </div>
          </div>
        </Panel>
      </div>
    </PageContainer>
  );
}

// Helper to emit audit events from other pages.
export function emitAudit(action: string, targetHash: string, actorHash: string, requestId: string) {
  const entry: AuditEntry = {
    timestamp: new Date().toISOString(),
    event_id: crypto.randomUUID(),
    action,
    target_hash: targetHash,
    actor_hash: actorHash,
    request_id: requestId,
    high_risk: HIGH_RISK_ACTIONS.includes(action),
  };
  window.dispatchEvent(new CustomEvent("kvlt-audit-event", { detail: entry }));
}
