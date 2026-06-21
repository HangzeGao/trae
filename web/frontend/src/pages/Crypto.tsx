import { useState } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { Lock, Unlock, FileCode2, ArrowRight } from "lucide-react";
import { api } from "../lib/api";
import { useAuth } from "../lib/store";
import { PageContainer, Panel, SuiteBadge, MonoReadout, showToast } from "../components";
import { parseEnvelope, toBase64, fromBase64 } from "../lib/envelope";
import type { KeyDTO, EncryptResponse, DecryptResponse, ParsedEnvelope } from "../lib/types";

export function CryptoPage() {
  const { tenantId } = useAuth();
  const [keyId, setKeyId] = useState("");
  const [plaintext, setPlaintext] = useState("hello, cryogenic vault");
  const [encPurpose, setEncPurpose] = useState("sandbox");
  const [encResource, setEncResource] = useState("");
  const [encResult, setEncResult] = useState<EncryptResponse | null>(null);

  const [decCiphertext, setDecCiphertext] = useState("");
  const [decPurpose, setDecPurpose] = useState("");
  const [decResource, setDecResource] = useState("");
  const [decResult, setDecResult] = useState<DecryptResponse | null>(null);

  const [parseInput, setParseInput] = useState("");
  const [parsed, setParsed] = useState<ParsedEnvelope | null>(null);

  const { data: keysData } = useQuery({
    queryKey: ["keys", tenantId],
    queryFn: () => api.get<{ keys: KeyDTO[] }>("/v1/keys"),
  });
  const keys = (keysData?.keys ?? []).filter((k) => k.status === "ACTIVE");

  const encMut = useMutation({
    mutationFn: () =>
      api.post<EncryptResponse>("/v1/crypto/encrypt", {
        tenant_id: tenantId,
        key_id: keyId,
        plaintext: toBase64(plaintext),
        aad: { purpose: encPurpose || undefined, resource_id: encResource || undefined },
      }),
    onSuccess: (r) => { setEncResult(r); showToast("encrypted", "success"); },
    onError: (e: Error) => showToast(e.message, "error"),
  });

  const decMut = useMutation({
    mutationFn: () =>
      api.post<DecryptResponse>("/v1/crypto/decrypt", {
        tenant_id: tenantId,
        ciphertext: decCiphertext,
        aad: { purpose: decPurpose || undefined, resource_id: decResource || undefined },
      }),
    onSuccess: (r) => { setDecResult(r); showToast("decrypted", "success"); },
    onError: (e: Error) => showToast(e.message, "error"),
  });

  const doParse = () => {
    setParsed(parseEnvelope(parseInput));
  };

  return (
    <PageContainer>
      <h1 className="section-title">crypto sandbox</h1>
      <p className="section-subtitle">interactive encrypt / decrypt · envelope v1 inspector</p>

      <div className="grid-2" style={{ marginBottom: 24 }}>
        {/* Encrypt */}
        <Panel title="Encrypt">
          <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
            <div>
              <label className="input-label">Key</label>
              <select className="select" value={keyId} onChange={(e) => setKeyId(e.target.value)}>
                <option value="">— select key —</option>
                {keys.map((k) => (
                  <option key={k.key_id} value={k.key_id}>
                    {k.name} ({k.suite_id})
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="input-label">Plaintext</label>
              <textarea
                className="textarea"
                value={plaintext}
                onChange={(e) => setPlaintext(e.target.value)}
                placeholder="enter plaintext..."
              />
            </div>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 10 }}>
              <div>
                <label className="input-label">AAD Purpose</label>
                <input className="input" value={encPurpose} onChange={(e) => setEncPurpose(e.target.value)} />
              </div>
              <div>
                <label className="input-label">AAD Resource ID</label>
                <input className="input" value={encResource} onChange={(e) => setEncResource(e.target.value)} />
              </div>
            </div>
            <button className="btn btn-primary" disabled={!keyId || encMut.isPending} onClick={() => encMut.mutate()}>
              <Lock size={14} /> {encMut.isPending ? "Encrypting..." : "Encrypt"}
            </button>
            {encResult && (
              <div style={{ marginTop: 4 }}>
                <div style={{ display: "flex", gap: 8, marginBottom: 10, alignItems: "center" }}>
                  <SuiteBadge suite={encResult.suite_id} />
                  <span className="mono" style={{ fontSize: 11, color: "var(--text-tertiary)" }}>
                    key v{encResult.key_version}
                  </span>
                </div>
                <MonoReadout label="Ciphertext (base64 envelope)" value={encResult.ciphertext} copyable />
                <button
                  className="btn btn-ghost btn-sm"
                  style={{ marginTop: 8 }}
                  onClick={() => { setDecCiphertext(encResult.ciphertext); setDecPurpose(encPurpose); setDecResource(encResource); }}
                >
                  <ArrowRight size={12} /> Send to decrypt
                </button>
              </div>
            )}
          </div>
        </Panel>

        {/* Decrypt */}
        <Panel title="Decrypt">
          <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
            <div>
              <label className="input-label">Ciphertext (base64)</label>
              <textarea
                className="textarea"
                value={decCiphertext}
                onChange={(e) => setDecCiphertext(e.target.value)}
                placeholder="paste base64 envelope..."
                style={{ minHeight: 100 }}
              />
            </div>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 10 }}>
              <div>
                <label className="input-label">AAD Purpose</label>
                <input className="input" value={decPurpose} onChange={(e) => setDecPurpose(e.target.value)} />
              </div>
              <div>
                <label className="input-label">AAD Resource ID</label>
                <input className="input" value={decResource} onChange={(e) => setDecResource(e.target.value)} />
              </div>
            </div>
            <button className="btn btn-primary" disabled={!decCiphertext || decMut.isPending} onClick={() => decMut.mutate()}>
              <Unlock size={14} /> {decMut.isPending ? "Decrypting..." : "Decrypt"}
            </button>
            {decResult && (
              <div style={{ marginTop: 4 }}>
                <div style={{ display: "flex", gap: 8, marginBottom: 10 }}>
                  <span className="mono" style={{ fontSize: 11, color: "var(--text-tertiary)" }}>
                    key: {decResult.key_id.slice(0, 16)}… v{decResult.key_version}
                  </span>
                </div>
                <MonoReadout label="Plaintext (decoded)" value={fromBase64(decResult.plaintext)} copyable />
                <div style={{ marginTop: 8 }}>
                  <MonoReadout label="Plaintext (base64)" value={decResult.plaintext} copyable />
                </div>
              </div>
            )}
          </div>
        </Panel>
      </div>

      {/* Envelope Inspector */}
      <Panel title="Envelope v1 Inspector">
        <div style={{ display: "flex", gap: 10, marginBottom: 14 }}>
          <input
            className="input"
            value={parseInput}
            onChange={(e) => setParseInput(e.target.value)}
            placeholder="paste base64 envelope to parse..."
          />
          <button className="btn btn-secondary" onClick={doParse} disabled={!parseInput.trim()}>
            <FileCode2 size={14} /> Parse
          </button>
        </div>
        {parseInput && !parsed && (
          <div style={{ color: "var(--danger)", fontFamily: '"JetBrains Mono", monospace', fontSize: 12 }}>
            invalid envelope — magic, version, or length check failed
          </div>
        )}
        {parsed && (
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
            <dl className="kv-list">
              {[
                ["magic", parsed.magic],
                ["version", `v${parsed.version}`],
                ["flags", `0x${parsed.flags.toString(16).padStart(4, "0")}`],
                ["suite_id", parsed.suite_id],
                ["key_id", parsed.key_id],
                ["key_version", `v${parsed.key_version}`],
                ["policy_version", `v${parsed.policy_version}`],
              ].map(([k, v]) => (
                <div key={k} style={{ display: "contents" }}>
                  <dt>{k}</dt>
                  <dd>{v}</dd>
                </div>
              ))}
            </dl>
            <dl className="kv-list">
              {[
                ["nonce_len", `${parsed.nonce_len} bytes`],
                ["tag_len", `${parsed.tag_len} bytes`],
                ["ciphertext_len", `${parsed.ciphertext_len} bytes`],
              ].map(([k, v]) => (
                <div key={k} style={{ display: "contents" }}>
                  <dt>{k}</dt>
                  <dd>{v}</dd>
                </div>
              ))}
              <dt>aad_hash</dt>
              <dd style={{ color: "var(--accent)" }}>{parsed.aad_hash}</dd>
              <dt>nonce</dt>
              <dd>{parsed.nonce}</dd>
              <dt>tag</dt>
              <dd>{parsed.tag}</dd>
            </dl>
          </div>
        )}
      </Panel>
    </PageContainer>
  );
}
