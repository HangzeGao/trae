import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams, useNavigate } from "react-router-dom";
import { ArrowLeft, Ban, CheckCircle, RefreshCw, Trash2, AlertTriangle } from "lucide-react";
import { api } from "../lib/api";
import { PageContainer, Panel, StatusPill, SuiteBadge, Loading, ErrorState, Modal, KVList, showToast } from "../components";
import type { KeyDTO } from "../lib/types";

export function KeyDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [confirmDestroy, setConfirmDestroy] = useState(false);

  const { data: key, isLoading, error } = useQuery({
    queryKey: ["key", id],
    queryFn: () => api.get<KeyDTO>(`/v1/keys/${id}`),
    enabled: !!id,
  });

  const invalidate = () => qc.invalidateQueries({ queryKey: ["key", id] });

  const disableMut = useMutation({
    mutationFn: () => api.post(`/v1/keys/${id}/disable`),
    onSuccess: () => { invalidate(); showToast("key disabled", "success"); },
    onError: (e: Error) => showToast(e.message, "error"),
  });
  const enableMut = useMutation({
    mutationFn: () => api.post(`/v1/keys/${id}/enable`),
    onSuccess: () => { invalidate(); showToast("key enabled", "success"); },
    onError: (e: Error) => showToast(e.message, "error"),
  });
  const rotateMut = useMutation({
    mutationFn: () => api.post(`/v1/keys/${id}/rotate`),
    onSuccess: () => { invalidate(); showToast("key rotated", "success"); },
    onError: (e: Error) => showToast(e.message, "error"),
  });
  const destroyMut = useMutation({
    mutationFn: () => api.post(`/v1/keys/${id}/schedule-destroy`),
    onSuccess: () => { invalidate(); setConfirmDestroy(false); showToast("destroy scheduled", "warning"); },
    onError: (e: Error) => showToast(e.message, "error"),
  });

  if (isLoading) return <PageContainer><Loading label="loading key..." /></PageContainer>;
  if (error) return <PageContainer><ErrorState message={error.message} /></PageContainer>;
  if (!key) return <PageContainer><ErrorState message="key not found" /></PageContainer>;

  return (
    <PageContainer>
      <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 20 }}>
        <button className="btn btn-ghost btn-sm" onClick={() => navigate("/ui/keys")}>
          <ArrowLeft size={14} /> Keys
        </button>
      </div>

      <div style={{ display: "flex", alignItems: "center", gap: 16, marginBottom: 24 }}>
        <h1 className="section-title" style={{ marginBottom: 0 }}>{key.name}</h1>
        <StatusPill status={key.status} />
        <SuiteBadge suite={key.suite_id} />
        <span className="mono" style={{ color: "var(--text-tertiary)", fontSize: 12 }}>v{key.current_version}</span>
      </div>

      <div className="grid-2" style={{ marginBottom: 24 }}>
        <Panel title="Metadata">
          <KVList
            items={[
              ["Key ID", key.key_id],
              ["Tenant", key.tenant_id],
              ["Purpose", key.purpose],
              ["Policy", key.policy_id],
              ["Suite", key.suite_id],
              ["Version", `v${key.current_version}`],
              ["Status", key.status],
              ["Created", new Date(key.created_at).toLocaleString()],
            ]}
          />
        </Panel>

        <Panel title="State Machine Operations">
          <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
            <p style={{ fontSize: 12, color: "var(--text-secondary)", marginBottom: 8 }}>
              Available actions depend on the current key state.
            </p>
            {key.status === "ACTIVE" && (
              <>
                <button className="btn btn-secondary" onClick={() => disableMut.mutate()} disabled={disableMut.isPending}>
                  <Ban size={14} /> Disable
                </button>
                <button className="btn btn-secondary" onClick={() => rotateMut.mutate()} disabled={rotateMut.isPending}>
                  <RefreshCw size={14} /> Rotate
                </button>
                <button className="btn btn-danger" onClick={() => setConfirmDestroy(true)}>
                  <Trash2 size={14} /> Schedule Destroy
                </button>
              </>
            )}
            {key.status === "DISABLED" && (
              <button className="btn btn-primary" onClick={() => enableMut.mutate()} disabled={enableMut.isPending}>
                <CheckCircle size={14} /> Enable
              </button>
            )}
            {key.status === "DESTROY_PENDING" && (
              <div style={{ display: "flex", gap: 8, alignItems: "center", color: "var(--warning)", fontSize: 12, fontFamily: '"JetBrains Mono", monospace' }}>
                <AlertTriangle size={14} />
                destroy pending — decryption only, no new encryption
              </div>
            )}
            {key.status === "DESTROYED" && (
              <div style={{ display: "flex", gap: 8, alignItems: "center", color: "var(--danger)", fontSize: 12, fontFamily: '"JetBrains Mono", monospace' }}>
                <AlertTriangle size={14} />
                key destroyed — permanently inaccessible
              </div>
            )}
          </div>
        </Panel>
      </div>

      {confirmDestroy && (
        <Modal
          title="Confirm Destroy"
          danger
          onClose={() => setConfirmDestroy(false)}
          onConfirm={() => destroyMut.mutate()}
          confirmLabel="Schedule Destroy"
        >
          <p style={{ fontSize: 13, color: "var(--text-secondary)" }}>
            This will schedule <strong style={{ color: "var(--text-primary)" }}>{key.name}</strong> for destruction.
            After the grace period, the key material will be permanently erased.
            Decryption remains possible until final destruction.
          </p>
        </Modal>
      )}
    </PageContainer>
  );
}
