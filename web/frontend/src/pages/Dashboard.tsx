import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { KeyRound, Server, ShieldCheck, Activity } from "lucide-react";
import { api, healthCheck } from "../lib/api";
import { useAuth } from "../lib/store";
import { PageContainer, Panel, StatusPill, SuiteBadge, Loading } from "../components";
import type { KeyDTO } from "../lib/types";

export function DashboardPage() {
  const { tenantId } = useAuth();

  const { data: keysData, isLoading: keysLoading } = useQuery({
    queryKey: ["keys", tenantId],
    queryFn: () => api.get<{ keys: KeyDTO[] }>("/ui/api/v1/keys"),
  });

  const { data: health } = useQuery({
    queryKey: ["health"],
    queryFn: healthCheck,
    refetchInterval: 5000,
  });

  const keys = keysData?.keys ?? [];
  const activeKeys = keys.filter((k) => k.status === "ACTIVE").length;
  const disabledKeys = keys.filter((k) => k.status === "DISABLED").length;
  const pendingDestroy = keys.filter((k) => k.status === "DESTROY_PENDING").length;

  const recentKeys = [...keys]
    .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
    .slice(0, 5);

  return (
    <PageContainer>
      <h1 className="section-title">dashboard</h1>
      <p className="section-subtitle">system overview · {tenantId}</p>

      <div className="grid-stats" style={{ marginBottom: 24 }}>
        <StatCard
          icon={<KeyRound />}
          label="Keys"
          value={keys.length}
          sub={[
            { label: "active", value: activeKeys, color: "var(--success)" },
            { label: "disabled", value: disabledKeys, color: "var(--warning)" },
            { label: "destroy", value: pendingDestroy, color: "var(--danger)" },
          ]}
        />
        <StatCard icon={<Server />} label="Nodes" value="—" sub={[]} />
        <StatCard icon={<ShieldCheck />} label="CRK Version" value="1" sub={[{ label: "status", value: "active", color: "var(--success)" }]} />
        <StatCard
          icon={<Activity />}
          label="Health"
          value={health?.status === "ok" ? "OK" : "—"}
          sub={health?.status === "ok" ? [{ label: "api", value: "online", color: "var(--success)" }] : []}
        />
      </div>

      <Panel title="Recent Keys" action={<Link to="/ui/keys" className="btn btn-ghost btn-sm">View all →</Link>}>
        {keysLoading ? (
          <Loading label="loading keys..." />
        ) : recentKeys.length === 0 ? (
          <div style={{ color: "var(--text-tertiary)", padding: 20, textAlign: "center", fontFamily: '"JetBrains Mono", monospace', fontSize: 12 }}>
            no keys yet — create one from the keys page
          </div>
        ) : (
          <table className="data-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Key ID</th>
                <th>Suite</th>
                <th>Ver</th>
                <th>Status</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {recentKeys.map((k) => (
                <tr key={k.key_id} onClick={() => (window.location.href = `/ui/keys/${k.key_id}`)}>
                  <td>{k.name}</td>
                  <td className="mono">{k.key_id.slice(0, 16)}...</td>
                  <td><SuiteBadge suite={k.suite_id} /></td>
                  <td className="mono">v{k.current_version}</td>
                  <td><StatusPill status={k.status} /></td>
                  <td className="mono">{new Date(k.created_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Panel>
    </PageContainer>
  );
}

function StatCard({
  icon,
  label,
  value,
  sub,
}: {
  icon: React.ReactNode;
  label: string;
  value: string | number;
  sub: { label: string; value: string | number; color: string }[];
}) {
  return (
    <div className="stat-card">
      <div className="stat-label" style={{ display: "flex", alignItems: "center", gap: 6 }}>
        <span style={{ opacity: 0.5 }}>{icon}</span>
        {label}
      </div>
      <div className="stat-value">{value}</div>
      {sub.length > 0 && (
        <div className="stat-sub">
          {sub.map((s) => (
            <span key={s.label} style={{ color: s.color }}>
              {s.value} {s.label}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
