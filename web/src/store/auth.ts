import { defineStore } from 'pinia';

export interface AuthState {
  token: string | null;
  role: string | null;
  user: any;
}

export const useAuthStore = defineStore('auth', {
  state: (): AuthState => ({
    token: localStorage.getItem('token') || null,
    role: localStorage.getItem('role') || null,
    user: null,
  }),
  actions: {
    setToken(token: string, role: string) {
      this.token = token;
      this.role = role;
      localStorage.setItem('token', token);
      localStorage.setItem('role', role);
    },
    logout() {
      this.token = null;
      this.role = null;
      localStorage.removeItem('token');
      localStorage.removeItem('role');
    },
  },
});
