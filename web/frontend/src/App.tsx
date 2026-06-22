import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useAuth } from "./lib/store";
import { ToastContainer } from "./components/ui";
import { LoginPage } from "./pages/Login";
import { DashboardPage } from "./pages/Dashboard";
import { KeysPage } from "./pages/Keys";
import { KeyDetailPage } from "./pages/KeyDetail";
import { NodesPage } from "./pages/Nodes";
import { NodeDetailPage } from "./pages/NodeDetail";
import { CryptoPage } from "./pages/Crypto";
import { DataKeysPage } from "./pages/DataKeys";
import { PolicyPage } from "./pages/Policy";
import { AuditPage } from "./pages/Audit";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, refetchOnWindowFocus: false },
  },
});

function ProtectedRoutes() {
  const token = useAuth((s) => s.token);
  if (!token) return <Navigate to="/ui/login" replace />;

  return (
    <Routes>
      <Route path="dashboard" element={<DashboardPage />} />
      <Route path="keys" element={<KeysPage />} />
      <Route path="keys/:id" element={<KeyDetailPage />} />
      <Route path="nodes" element={<NodesPage />} />
      <Route path="nodes/:id" element={<NodeDetailPage />} />
      <Route path="crypto" element={<CryptoPage />} />
      <Route path="data-keys" element={<DataKeysPage />} />
      <Route path="policy" element={<PolicyPage />} />
      <Route path="audit" element={<AuditPage />} />
      <Route path="*" element={<Navigate to="dashboard" replace />} />
    </Routes>
  );
}

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/ui/login" element={<LoginPage />} />
          <Route path="/ui/*" element={<ProtectedRoutes />} />
          <Route path="*" element={<Navigate to="/ui/dashboard" replace />} />
        </Routes>
      </BrowserRouter>
      <ToastContainer />
    </QueryClientProvider>
  );
}
