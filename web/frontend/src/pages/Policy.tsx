import { PageContainer, Panel } from "../components";
import { Shield } from "lucide-react";

interface SuiteDef {
  id: string;
  algorithm: string;
  keyBits: number;
  mode: string;
  status: "active" | "decrypt_only" | "disabled";
}

const suites: SuiteDef[] = [
  { id: "AES_256_GCM", algorithm: "AES", keyBits: 256, mode: "GCM", status: "active" },
  { id: "SM4_GCM", algorithm: "SM4", keyBits: 128, mode: "GCM", status: "active" },
  { id: "AES_128_GCM", algorithm: "AES", keyBits: 128, mode: "GCM", status: "active" },
  { id: "AES_256_CBC_HMAC_SHA256", algorithm: "AES", keyBits: 256, mode: "CBC", status: "decrypt_only" },
  { id: "AES_128_CBC_HMAC_SHA256", algorithm: "AES", keyBits: 128, mode: "CBC", status: "decrypt_only" },
];

const statusColor: Record<string, string> = {
  active: "var(--success)",
  decrypt_only: "var(--warning)",
  disabled: "var(--danger)",
};

export function PolicyPage() {
  return (
    <PageContainer>
      <h1 className="section-title">policy</h1>
      <p className="section-subtitle">crypto policy engine · suite status matrix · default-v1</p>

      <Panel
        title="Suite Matrix"
        action={
          <div style={{ display: "flex", gap: 12, fontSize: 10, fontFamily: '"JetBrains Mono", monospace' }}>
            {Object.entries(statusColor).map(([s, c]) => (
              <span key={s} style={{ color: c, display: "flex", alignItems: "center", gap: 4 }}>
                <span style={{ width: 6, height: 6, borderRadius: "50%", background: c }} />
                {s}
              </span>
            ))}
          </div>
        }
      >
        <table className="data-table">
          <thead>
            <tr>
              <th>Suite ID</th>
              <th>Algorithm</th>
              <th>Key Bits</th>
              <th>Mode</th>
              <th>Status</th>
              <th>Encrypt</th>
              <th>Decrypt</th>
            </tr>
          </thead>
          <tbody>
            {suites.map((s) => (
              <tr key={s.id} style={{ cursor: "default" }}>
                <td className="mono" style={{ fontWeight: 500 }}>{s.id}</td>
                <td className="mono">{s.algorithm}</td>
                <td className="mono">{s.keyBits}</td>
                <td><span className="suite-badge">{s.mode}</span></td>
                <td>
                  <span style={{ color: statusColor[s.status], fontFamily: '"JetBrains Mono", monospace', fontSize: 11, textTransform: "uppercase", letterSpacing: "0.08em" }}>
                    ● {s.status}
                  </span>
                </td>
                <td className="mono" style={{ color: s.status === "active" ? "var(--success)" : "var(--text-tertiary)" }}>
                  {s.status === "active" ? "✓" : "✗"}
                </td>
                <td className="mono" style={{ color: s.status === "disabled" ? "var(--text-tertiary)" : "var(--success)" }}>
                  {s.status === "disabled" ? "✗" : "✓"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Panel>

      <div style={{ marginTop: 16 }}>
        <Panel title="Policy Rules">
          <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
            {[
              "Default suite for new keys: AES_256_GCM",
              "GCM mode suites are active for encrypt + decrypt",
              "CBC mode suites are decrypt_only (legacy data migration)",
              "ECB mode is permanently blocked",
              "New keys cannot be created with decrypt_only or disabled suites",
              "Suite deprecation requires policy version bump",
            ].map((rule, i) => (
              <div
                key={i}
                style={{
                  display: "flex",
                  gap: 10,
                  alignItems: "flex-start",
                  padding: "10px 14px",
                  background: "var(--bg-inset)",
                  border: "1px solid var(--border)",
                  borderRadius: 2,
                }}
              >
                <Shield size={14} style={{ color: "var(--accent)", marginTop: 1, flexShrink: 0 }} />
                <span style={{ fontSize: 12, fontFamily: '"JetBrains Mono", monospace', color: "var(--text-secondary)" }}>
                  {rule}
                </span>
              </div>
            ))}
          </div>
        </Panel>
      </div>
    </PageContainer>
  );
}
