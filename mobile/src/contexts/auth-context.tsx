// Extracted from: web/src/contexts/auth-context.tsx — ported to mobile
// Differences: uses mobile api client (Bearer token via SecureStore), removeToken on logout

import {
  createContext,
  useContext,
  useEffect,
  useState,
  useCallback,
  type ReactNode,
} from "react";
import { api } from "../lib/api";
import { mobileAdapter } from "../lib/api-adapter";
import type { User } from "../../../packages/shared/src/types";

interface AuthState {
  user: User | null;
  loading: boolean;
  logout: () => Promise<void>;
  refreshUser: () => Promise<void>;
}

const AuthContext = createContext<AuthState>({
  user: null,
  loading: true,
  logout: async () => {},
  refreshUser: async () => {},
});

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api
      .fetchMe()
      .then(setUser)
      .catch(() => setUser(null))
      .finally(() => setLoading(false));
  }, []);

  const logout = useCallback(async () => {
    await api.logout();
    await mobileAdapter.removeToken();
    setUser(null);
  }, []);

  const refreshUser = useCallback(async () => {
    try {
      const updatedUser = await api.fetchMe();
      setUser(updatedUser);
    } catch {
      // Silently fail — user might be logged out
    }
  }, []);

  return (
    <AuthContext.Provider value={{ user, loading, logout, refreshUser }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}
