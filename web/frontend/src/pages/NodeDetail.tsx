import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams, useNavigate } from "react-router-dom";
import { ArrowLeft, CheckCircle, Ban, ShieldCheck, ShieldX } from "lucide-react";
import { api } from "../lib/api";
import { PageContainer, Panel, StatusPill, Loading, ErrorState, KVList, showToast } from "../components";
import type { NodeDTO } from "../lib/types";

export function NodeDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const qc = useQueryClient();

  const { data: node, isLoading, error } = useQuery({
    queryKey: ["node", id],
    queryFn: () => api.get<NodeDTO>(`/ui/api/v1/nodes/${id}`),
    enabled: !!id,
  });

  const markReadyMut = useMutation({
    mutationFn: () => api.post<NodeDTO>(`/ui/api/v1/nodes/${id}/mark-ready`),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["node", id] }); showToast("node marked ready", "success"); },
    onError: (e: Error) => showToast(e.message, "error"),
  });
  const revokeMut = useMutation({
    mutationFn: () => api.post(`/ui/api/v1/nodes/${id}/revoke`),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["node", id] }); showToast("node revoked", "warning"); },
    onError: (e: Error) => showToast(e.message, "error"),
  });

  if (isLoading) return <PageContainer><Loading label="loading node..." /></PageContainer>;
  if (error) return <PageContainer><ErrorState message={error.message} /></PageContainer>;
  if (!node) return <PageContainer><ErrorState message="node not found" /></PageContainer>;

  const baselineChecks = [
    { label: "SELinux", value: node.baseline?.selinux_status ?? "unknown", ok: node.baseline?.selinux_status === "enforcing" },
    { label: "Kernel", value: node.baseline?.kernel_version ?? "—", ok: true },
    { label: "Virt Platform", value: node.baseline?.virt_platform ?? "—", ok: node.baseline?.virt_platform === "kvm" },
    { label: "TPM2-TSS", value: node.baseline?.tpm2_tss_version ?? "—", ok: true },
    { label: "swtpm Isolated", value: node.baseline?.swtpm_isolated ? "yes" : "no", ok: node.baseline?.swtpm_isolated },
  ];

  return (
    <PageContainer>
      <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 20 }}>
        <button className="btn btn-ghost btn-sm" onClick={() => navigate("/ui/nodes")}>
          <ArrowLeft size={14} /> Nodes
        </button>
      </div>

      <div style={{ display: "flex", alignItems: "center", gap: 16, marginBottom: 24 }}>
        <h1 className="section-title" style={{ marginBottom: 0 }}>{node.node_id}</h1>
        <StatusPill status={node.status} />
        <span className="suite-badge">{node.role}</span>
      </div>

      <div className="grid-2" style={{ marginBottom: 24 }}>
        <Panel title="Node Info">
          <KVList
            items={[
              ["Node ID", node.node_id],
              ["Role", node.role],
              ["Status", node.status],
              ["Cluster Epoch", node.cluster_epoch],
              ["Attestation Epoch", node.attestation_epoch],
              ["Ready Reason", node.ready_reason || "—"],
              ["Created", new Date(node.created_at).toLocaleString()],
              ["Updated", new Date(node.updated_at).toLocaleString()],
            ]}
          />
        </Panel>

        <Panel title="Security Baseline">
          <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
            {baselineChecks.map((c) => (
              <div
                key={c.label}
                style={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  padding: "10px 14px",
                  background: "var(--bg-inset)",
                  border: "1px solid var(--border)",
                  borderRadius: 2,
                }}
              >
                <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                  {c.ok ? (
                    <ShieldCheck size={15} style={{ color: "var(--success)" }} />
                  ) : (
                    <ShieldX size={15} style={{ color: "var(--danger)" }} />
                  )}
                  <span style={{ fontFamily: '"JetBrains Mono", monospace', fontSize: 12, color: "var(--text-secondary)" }}>
                    {c.label}
                  </span>
                </div>
                <span className="mono" style={{ fontSize: 12, color: c.ok ? "var(--text-primary)" : "var(--danger)" }}>
                  {c.value}
                </span>
              </div>
            ))}
          </div>
        </Panel>
      </div>

      <Panel title="State Machine Operations">
        <div style={{ display: "flex", gap: 10 }}>
          {node.status === "REGISTERED" && (
            <button className="btn btn-primary" onClick={() => markReadyMut.mutate()} disabled={markReadyMut.isPending}>
              <CheckCircle size={14} /> Mark Ready
            </button>
          )}
          {(node.status === "READY" || node.status === "DEGRADED") && (
            <button className="btn btn-danger" onClick={() => revokeMut.mutate()} disabled={revokeMut.isPending}>
              <Ban size={14} /> Revoke
            </button>
          )}
          {node.status === "REVOKED" && (
            <span style={{ color: "var(--danger)", fontFamily: '"JetBrains Mono", monospace', fontSize: 12 }}>
              node revoked — permanently removed from cluster
            </span>
          )}
        </div>
      </Panel>
    </PageContainer>
  );
}
