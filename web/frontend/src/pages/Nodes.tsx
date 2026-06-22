import { useState } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { Plus, Server } from "lucide-react";
import { api } from "../lib/api";
import { PageContainer, Panel, StatusPill, Loading, Modal, showToast } from "../components";
import type { NodeDTO, RegisterNodeReq } from "../lib/types";

export function NodesPage() {
  const navigate = useNavigate();
  const [showRegister, setShowRegister] = useState(false);

  // Nodes list isn't exposed as a list endpoint in P0, so we track registered
  // node IDs via localStorage for the UI. In a real deployment this would be
  // GET /v1/nodes. For P0 demo, we show nodes registered in this session.
  const [knownNodes, setKnownNodes] = useState<string[]>(() => {
    try {
      return JSON.parse(localStorage.getItem("kvlt-nodes") ?? "[]");
    } catch {
      return [];
    }
  });

  const queries = useQuery({
    queryKey: ["nodes", knownNodes],
    queryFn: async () => {
      const results = await Promise.allSettled(
        knownNodes.map((id) => api.get<NodeDTO>(`/ui/api/v1/nodes/${id}`))
      );
      return results
        .filter((r): r is PromiseFulfilledResult<NodeDTO> => r.status === "fulfilled")
        .map((r) => r.value);
    },
    enabled: knownNodes.length > 0,
  });

  const nodes = queries.data ?? [];
  const registerMut = useMutation({
    mutationFn: (req: RegisterNodeReq) => api.post<NodeDTO>("/ui/api/v1/nodes/register", req),
    onSuccess: (node) => {
      const updated = [...new Set([...knownNodes, node.node_id])];
      setKnownNodes(updated);
      localStorage.setItem("kvlt-nodes", JSON.stringify(updated));
      setShowRegister(false);
      showToast("node registered", "success");
    },
    onError: (e: Error) => showToast(e.message, "error"),
  });

  return (
    <PageContainer>
      <h1 className="section-title">nodes</h1>
      <p className="section-subtitle">compute node registry · attestation baseline</p>

      <Panel
        title="Node Registry"
        action={
          <button className="btn btn-primary btn-sm" onClick={() => setShowRegister(true)}>
            <Plus size={14} /> Register Node
          </button>
        }
      >
        {queries.isLoading ? (
          <Loading label="loading nodes..." />
        ) : nodes.length === 0 ? (
          <div style={{ textAlign: "center", padding: 40, color: "var(--text-tertiary)" }}>
            <Server size={28} style={{ opacity: 0.3, marginBottom: 10 }} />
            <div style={{ fontFamily: '"JetBrains Mono", monospace', fontSize: 12 }}>no nodes registered</div>
          </div>
        ) : (
          <table className="data-table">
            <thead>
              <tr>
                <th>Node ID</th>
                <th>Role</th>
                <th>Status</th>
                <th>Cluster Epoch</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {nodes.map((n) => (
                <tr key={n.node_id} onClick={() => navigate(`/ui/nodes/${n.node_id}`)}>
                  <td className="mono">{n.node_id}</td>
                  <td className="mono" style={{ fontSize: 12 }}>{n.role}</td>
                  <td><StatusPill status={n.status} /></td>
                  <td className="mono">{n.cluster_epoch}</td>
                  <td className="mono" style={{ fontSize: 11 }}>{new Date(n.created_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Panel>

      {showRegister && (
        <RegisterNodeModal
          onClose={() => setShowRegister(false)}
          onRegister={(req) => registerMut.mutate(req)}
          loading={registerMut.isPending}
        />
      )}
    </PageContainer>
  );
}

function RegisterNodeModal({
  onClose,
  onRegister,
  loading,
}: {
  onClose: () => void;
  onRegister: (req: RegisterNodeReq) => void;
  loading: boolean;
}) {
  const [nodeId, setNodeId] = useState("");
  const [role, setRole] = useState("data");
  const [selinux, setSelinux] = useState("enforcing");
  const [kernel, setKernel] = useState("5.15.0");
  const [virt, setVirt] = useState("kvm");
  const [tss, setTss] = useState("3.2.0");
  const [swtpm, setSwtpm] = useState(true);

  return (
    <Modal title="Register Node" onClose={onClose}>
      <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        <div>
          <label className="input-label">Node ID</label>
          <input className="input" value={nodeId} onChange={(e) => setNodeId(e.target.value)} placeholder="node-1" autoFocus />
        </div>
        <div>
          <label className="input-label">Role</label>
          <select className="select" value={role} onChange={(e) => setRole(e.target.value)}>
            <option value="data">data</option>
            <option value="key">key</option>
            <option value="management">management</option>
          </select>
        </div>
        <div style={{ borderTop: "1px solid var(--border)", paddingTop: 12, marginTop: 4 }}>
          <div className="input-label" style={{ marginBottom: 10 }}>Security Baseline</div>
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 10 }}>
            <div>
              <label className="input-label">SELinux</label>
              <input className="input" value={selinux} onChange={(e) => setSelinux(e.target.value)} />
            </div>
            <div>
              <label className="input-label">Kernel</label>
              <input className="input" value={kernel} onChange={(e) => setKernel(e.target.value)} />
            </div>
            <div>
              <label className="input-label">Virt Platform</label>
              <input className="input" value={virt} onChange={(e) => setVirt(e.target.value)} />
            </div>
            <div>
              <label className="input-label">TPM2-TSS</label>
              <input className="input" value={tss} onChange={(e) => setTss(e.target.value)} />
            </div>
          </div>
          <label style={{ display: "flex", alignItems: "center", gap: 8, marginTop: 12, cursor: "pointer", fontSize: 13 }}>
            <input type="checkbox" checked={swtpm} onChange={(e) => setSwtpm(e.target.checked)} />
            swtpm isolated
          </label>
        </div>
      </div>
      <div style={{ marginTop: 20, display: "flex", justifyContent: "flex-end", gap: 10 }}>
        <button className="btn btn-secondary btn-sm" onClick={onClose}>Cancel</button>
        <button
          className="btn btn-primary btn-sm"
          disabled={!nodeId.trim() || loading}
          onClick={() =>
            onRegister({
              node_id: nodeId.trim(),
              role,
              baseline: {
                selinux_status: selinux,
                kernel_version: kernel,
                virt_platform: virt,
                tpm2_tss_version: tss,
                swtpm_isolated: swtpm,
              },
            })
          }
        >
          {loading ? "Registering..." : "Register"}
        </button>
      </div>
    </Modal>
  );
}
