import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { Plus, KeyRound } from "lucide-react";
import { api } from "../lib/api";
import { useAuth } from "../lib/store";
import { PageContainer, Panel, StatusPill, SuiteBadge, Loading, Modal, showToast } from "../components";
import type { KeyDTO, CreateKeyReq } from "../lib/types";

export function KeysPage() {
  const { tenantId } = useAuth();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [showCreate, setShowCreate] = useState(false);

  const { data, isLoading, error } = useQuery({
    queryKey: ["keys", tenantId],
    queryFn: () => api.get<{ keys: KeyDTO[] }>("/v1/keys"),
  });

  const createMut = useMutation({
    mutationFn: (req: CreateKeyReq) => api.post<KeyDTO>("/v1/keys", req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["keys", tenantId] });
      setShowCreate(false);
      showToast("key created", "success");
    },
    onError: (e: Error) => showToast(e.message, "error"),
  });

  const keys = data?.keys ?? [];

  return (
    <PageContainer>
      <h1 className="section-title">keys</h1>
      <p className="section-subtitle">cryptographic key management · {tenantId}</p>

      <Panel
        title="Key Inventory"
        action={
          <button className="btn btn-primary btn-sm" onClick={() => setShowCreate(true)}>
            <Plus size={14} /> Create Key
          </button>
        }
      >
        {isLoading ? (
          <Loading label="loading keys..." />
        ) : error ? (
          <div style={{ color: "var(--danger)", fontFamily: '"JetBrains Mono", monospace', fontSize: 12 }}>
            {error.message}
          </div>
        ) : keys.length === 0 ? (
          <div style={{ textAlign: "center", padding: 40, color: "var(--text-tertiary)" }}>
            <KeyRound size={28} style={{ opacity: 0.3, marginBottom: 10 }} />
            <div style={{ fontFamily: '"JetBrains Mono", monospace', fontSize: 12 }}>no keys — create one to begin</div>
          </div>
        ) : (
          <table className="data-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Key ID</th>
                <th>Suite</th>
                <th>Version</th>
                <th>Status</th>
                <th>Purpose</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {keys.map((k) => (
                <tr key={k.key_id} onClick={() => navigate(`/ui/keys/${k.key_id}`)}>
                  <td style={{ fontWeight: 500 }}>{k.name}</td>
                  <td className="mono">{k.key_id.slice(0, 20)}…</td>
                  <td><SuiteBadge suite={k.suite_id} /></td>
                  <td className="mono">v{k.current_version}</td>
                  <td><StatusPill status={k.status} /></td>
                  <td className="mono" style={{ fontSize: 12 }}>{k.purpose}</td>
                  <td className="mono" style={{ fontSize: 11 }}>{new Date(k.created_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Panel>

      {showCreate && (
        <CreateKeyModal
          tenantId={tenantId}
          onClose={() => setShowCreate(false)}
          onCreate={(req) => createMut.mutate(req)}
          loading={createMut.isPending}
        />
      )}
    </PageContainer>
  );
}

function CreateKeyModal({
  tenantId,
  onClose,
  onCreate,
  loading,
}: {
  tenantId: string;
  onClose: () => void;
  onCreate: (req: CreateKeyReq) => void;
  loading: boolean;
}) {
  const [name, setName] = useState("");
  const [purpose, setPurpose] = useState("encrypt_decrypt");
  const [suite, setSuite] = useState("AES_256_GCM");

  return (
    <Modal title="Create Key" onClose={onClose}>
      <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
        <div>
          <label className="input-label">Name</label>
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="my-app-key" autoFocus />
        </div>
        <div>
          <label className="input-label">Purpose</label>
          <select className="select" value={purpose} onChange={(e) => setPurpose(e.target.value)}>
            <option value="encrypt_decrypt">encrypt_decrypt</option>
            <option value="datakey">datakey</option>
            <option value="signing">signing</option>
          </select>
        </div>
        <div>
          <label className="input-label">Suite</label>
          <select className="select" value={suite} onChange={(e) => setSuite(e.target.value)}>
            <option value="AES_256_GCM">AES_256_GCM</option>
            <option value="SM4_GCM">SM4_GCM</option>
          </select>
        </div>
      </div>
      <div style={{ marginTop: 20, display: "flex", justifyContent: "flex-end", gap: 10 }}>
        <button className="btn btn-secondary btn-sm" onClick={onClose}>Cancel</button>
        <button
          className="btn btn-primary btn-sm"
          disabled={!name.trim() || loading}
          onClick={() => onCreate({ tenant_id: tenantId, name: name.trim(), purpose, policy_id: "default-v1", suite_id: suite })}
        >
          {loading ? "Creating..." : "Create"}
        </button>
      </div>
    </Modal>
  );
}
