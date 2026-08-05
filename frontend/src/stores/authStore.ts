import { create } from "zustand";
import api from "../api/client";

interface User {
  id: number;
  login: string;
  email: string;
  display_name: string;
  auth_source: string;
  is_admin: boolean;
}

interface AuthState {
  user: User | null;
  token: string | null;
  isLoggedIn: boolean;
  login: (email: string, password: string) => Promise<boolean>;
  setToken: (token: string) => void;
  logout: () => void;
  loadFromStorage: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  token: null,
  isLoggedIn: false,

  login: async (email, password) => {
    const res = await api.post("/api/auth/login", { email, password });
    const { token, user, must_change_password } = res.data.data;
    localStorage.setItem("token", token);
    localStorage.setItem("user", JSON.stringify(user));
    set({ user, token, isLoggedIn: true });
    return !!must_change_password; // true = 首登需改密
  },

  // 改密成功后用新 token 替换（旧 token 带首登标记）
  setToken: (token: string) => {
    localStorage.setItem("token", token);
    set({ token });
  },

  logout: () => {
    localStorage.removeItem("token");
    localStorage.removeItem("user");
    set({ user: null, token: null, isLoggedIn: false });
  },

  loadFromStorage: () => {
    const token = localStorage.getItem("token");
    const userStr = localStorage.getItem("user");
    if (token && userStr) {
      try {
        const user = JSON.parse(userStr);
        set({ user, token, isLoggedIn: true });
      } catch {
        localStorage.removeItem("token");
        localStorage.removeItem("user");
      }
    }
  },
}));
