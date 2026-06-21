import { create } from "zustand";
import { persist } from "zustand/middleware";

interface AuthState {
  token: string | null;
  tenantId: string;
  setToken: (token: string) => void;
  setTenantId: (id: string) => void;
  logout: () => void;
}

export const useAuth = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      tenantId: "t-default",
      setToken: (token) => set({ token }),
      setTenantId: (tenantId) => set({ tenantId }),
      logout: () => set({ token: null }),
    }),
    { name: "kvlt-auth" }
  )
);
