import { useState, useEffect } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { Key, Plus, Trash2, Timer } from "lucide-react";
import { api } from "../lib/api";
import { useAuth } from "../lib/store";
import { PageContainer, Panel, SuiteBadge, MonoReadout, showToast } from "../components";
import { bytesToHex } from "../lib/envelope";
import type { KeyDTO, DataKeyResponse } from "../lib/types";

export function DataKeysPage() {
  const { tenantId } = useAuth();
  const [keyId, setKeyId] = useState("");
  const [purpose, setPurpose] = useState("app-encryption");
  const [ttl, setTtl] = useState(300);
  const [ctx, setCtx] = useState<[string, string][]>([["app", "demo"], ["env", "test"]]);
  const [result, setResult] = useState<DataKeyResponse | null>(null);
  const [remaining, setRemaining] = useState<number | null>(null);

  const { data: keysData } = useQuery({
    queryKey: ["keys", tenantId],
    queryFn: () => api.get<{ keys: KeyDTO[] }>("/v1/keys"),
  });
  const keys = (keysData?.keys ?? []).filter((k) => k.status === "ACTIVE");

  const genMut = useMutation({
    mutationFn: () =>
      api.post<DataKeyResponse>("/v1/data-keys", {
        tenant_id: tenantId,
        key_id: keyId,
        purpose,
        ttl_seconds: ttl,
        encryption_context: Object.fromEntries(ctx.filter(([k]) => k)),
        caller: "direct",
      }),
    onSuccess: (r) => {
      setResult(r);
      showToast("data key generated", "success");
    },
    onError: (e: Error) => showToast(e.message, "error"),
  });

  // TTL countdown.
  useEffect(() => {
    if (!result) { setRemaining(null); return; }
    const target = new Date(result.client_zeroize_by).getTime();
    const tick = () => {
      const diff = Math.max(0, Math.floor((target - Date.now()) / 1000));
      setRemaining(diff);
    };
    tick();
    const iv = setInterval(tick, 1000);
    return () => clearInterval(iv);
  }, [result]);

  const addCtxRow = () => setCtx([...ctx, ["", ""]]);
  const updateCtx = (i: number, field: 0 | 1, val: string) => {
    const next = [...ctx];
    next[i][field] = val;
    setCtx(next);
  };
  const removeCtx = (i: number) => setCtx(ctx.filter((_, idx) => idx !== i));

  const fmtTTL = (s: number) => {
    const m = Math.floor(s / 60);
    const sec = s % 60;
    return `${m}:${sec.toString().padStart(2, "0")}`;
  };

  return (
    <PageContainer>
      <h1 className="section-title">data keys</h1>
      <p className="section-subtitle">envelope encryption · dek generation with ttl · ha-10</p>

      <div className="grid-2">
        <Panel title="Generate Data Key">
          <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
            <div>
              <label className="input-label">Wrapping Key</label>
              <select className="select" value={keyId} onChange={(e) => setKeyId(e.target.value)}>
                <option value="">— select key —</option>
                {keys.map((k) => (
                  <option key={k.key_id} value={k.key_id}>{k.name} ({k.suite_id})</option>
                ))}
              </select>
            </div>
            <div>
              <label className="input-label">Purpose</label>
              <input className="input" value={purpose} onChange={(e) => setPurpose(e.target.value)} />
            </div>
            <div>
              <label className="input-label">
                TTL · {fmtTTL(ttl)} · max 15:00
              </label>
              <input
                type="range"
                min={60}
                max={900}
                step={60}
                value={ttl}
                onChange={(e) => setTtl(Number(e.target.value))}
                style={{ width: "100%", accentColor: "var(--accent)" }}
              />
            </div>
            <div>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
                <label className="input-label" style={{ marginBottom: 0 }}>Encryption Context</label>
                <button className="btn btn-ghost btn-sm" onClick={addCtxRow}>
                  <Plus size={12} /> Add
                </button>
              </div>
              <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                {ctx.map(([k, v], i) => (
                  <div key={i} style={{ display: "flex", gap: 6 }}>
                    <input className="input" value={k} onChange={(e) => updateCtx(i, 0, e.target.value)} placeholder="key" />
                    <input className="input" value={v} onChange={(e) => updateCtx(i, 1, e.target.value)} placeholder="value" />
                    <button className="btn btn-ghost btn-sm" onClick={() => removeCtx(i)}>
                      <Trash2 size={12} />
                    </button>
                  </div>
                ))}
              </div>
            </div>
            <button className="btn btn-primary" disabled={!keyId || genMut.isPending} onClick={() => genMut.mutate()}>
              <Key size={14} /> {genMut.isPending ? "Generating..." : "Generate Data Key"}
            </button>
          </div>
        </Panel>

        <Panel title="Result">
          {!result ? (
            <div style={{ textAlign: "center", padding: 40, color: "var(--text-tertiary)" }}>
              <Key size={28} style={{ opacity: 0.3, marginBottom: 10 }} />
              <div style={{ fontFamily: '"JetBrains Mono", monospace', fontSize: 12 }}>
                generate a data key to see results
              </div>
            </div>
          ) : (
            <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
              <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
                <SuiteBadge suite={result.suite_id} />
                <span className="mono" style={{ fontSize: 11, color: "var(--text-tertiary)" }}>
                  key v{result.key_version}
                </span>
                {remaining !== null && (
                  <span
                    className="mono"
                    style={{
                      fontSize: 11,
                      marginLeft: "auto",
                      color: remaining < 60 ? "var(--danger)" : "var(--accent)",
                      display: "flex",
                      alignItems: "center",
                      gap: 4,
                    }}
                  >
                    <Timer size={11} /> zeroize in {fmtTTL(remaining)}
                  </span>
                )}
              </div>
              <MonoReadout label="Plaintext Data Key (base64)" value={result.plaintext_data_key} copyable />
              <MonoReadout label="Plaintext Data Key (hex)" value={bytesToHex(result.plaintext_data_key)} copyable />
              <MonoReadout label="Wrapped Data Key (base64)" value={result.wrapped_data_key} copyable />
              <MonoReadout label="Encryption Context Hash" value={result.encryption_context_hash} copyable />
              <div
                style={{
                  padding: "10px 14px",
                  background: "var(--warning-dim)",
                  border: "1px solid rgba(251,191,36,0.3)",
                  borderRadius: 2,
                  fontSize: 11,
                  fontFamily: '"JetBrains Mono", monospace',
                  color: "var(--warning)",
                  display: "flex",
                  gap: 8,
                }}
              >
                <Timer size={13} />
                client must zeroize plaintext data key before {new Date(result.client_zeroize_by).toLocaleTimeString()}
              </div>
            </div>
          )}
        </Panel>
      </div>
    </PageContainer>
  );
}
