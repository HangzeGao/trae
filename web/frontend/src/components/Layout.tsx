import { type ReactNode } from "react";
import { NavLink, useLocation } from "react-router-dom";
import {
  LayoutDashboard,
  KeyRound,
  Server,
  Lock,
  Key,
  Shield,
  ScrollText,
  LogOut,
} from "lucide-react";
import { useAuth } from "../lib/store";

interface NavEntry {
  to: string;
  label: string;
  icon: ReactNode;
}

const mgmtNav: NavEntry[] = [
  { to: "/ui/dashboard", label: "Dashboard", icon: <LayoutDashboard /> },
  { to: "/ui/keys", label: "Keys", icon: <KeyRound /> },
  { to: "/ui/nodes", label: "Nodes", icon: <Server /> },
];

const dataNav: NavEntry[] = [
  { to: "/ui/crypto", label: "Crypto Sandbox", icon: <Lock /> },
  { to: "/ui/data-keys", label: "Data Keys", icon: <Key /> },
];

const refNav: NavEntry[] = [
  { to: "/ui/policy", label: "Policy", icon: <Shield /> },
  { to: "/ui/audit", label: "Audit Log", icon: <ScrollText /> },
];

function NavGroup({ label, entries }: { label: string; entries: NavEntry[] }) {
  return (
    <div className="nav-group">
      <div className="nav-group-label">{label}</div>
      {entries.map((e) => (
        <NavLink key={e.to} to={e.to} className={({ isActive }) => `nav-item ${isActive ? "active" : ""}`}>
          {e.icon}
          {e.label}
        </NavLink>
      ))}
    </div>
  );
}

export function Sidebar() {
  const { token, tenantId, logout } = useAuth();
  return (
    <aside className="sidebar">
      <div className="brand">
        <div className="brand-title">kvlt</div>
        <div className="brand-sub">key vault · p0</div>
      </div>
      <div style={{ flex: 1, overflowY: "auto" }}>
        <NavGroup label="Management" entries={mgmtNav} />
        <NavGroup label="Data Plane" entries={dataNav} />
        <NavGroup label="Reference" entries={refNav} />
      </div>
      <div
        style={{
          padding: "12px 18px",
          borderTop: "1px solid var(--border)",
          fontSize: 11,
          fontFamily: '"JetBrains Mono", monospace',
          color: "var(--text-tertiary)",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
          <span>{tenantId}</span>
          {token && (
            <button
              onClick={logout}
              title="Logout"
              style={{
                background: "none",
                border: "none",
                color: "var(--text-tertiary)",
                cursor: "pointer",
                display: "flex",
                padding: 2,
              }}
            >
              <LogOut size={13} />
            </button>
          )}
        </div>
        <div style={{ marginTop: 4, color: "var(--success)", fontSize: 9 }}>
          ● authenticated
        </div>
      </div>
    </aside>
  );
}

export function Topbar({ children }: { children?: ReactNode }) {
  const location = useLocation();
  const path = location.pathname.replace("/ui/", "").replace("/ui", "");
  return (
    <div className="topbar">
      <div className="breadcrumb">
        <span>kvlt</span>
        <span>/</span>
        <span style={{ color: "var(--accent)" }}>{path || "dashboard"}</span>
      </div>
      <div style={{ flex: 1 }} />
      {children}
    </div>
  );
}

export function PageContainer({ children }: { children: ReactNode }) {
  return (
    <div className="app-shell">
      <Sidebar />
      <div className="main-area">
        <Topbar />
        <div className="content stagger">{children}</div>
      </div>
    </div>
  );
}
