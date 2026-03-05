import { create } from 'zustand'
import { authAPI } from '../api/client'

interface AuthState {
  token: string | null
  refreshToken: string | null
  isAuthenticated: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => void
  restore: () => void
}

export const useAuthStore = create<AuthState>((set) => ({
  token: null,
  refreshToken: null,
  isAuthenticated: false,

  login: async (username, password) => {
    const resp = await authAPI.login(username, password)
    localStorage.setItem('token', resp.access_token)
    localStorage.setItem('refreshToken', resp.refresh_token)
    set({ token: resp.access_token, refreshToken: resp.refresh_token, isAuthenticated: true })
  },

  logout: () => {
    localStorage.removeItem('token')
    localStorage.removeItem('refreshToken')
    set({ token: null, refreshToken: null, isAuthenticated: false })
  },

  restore: () => {
    const token = localStorage.getItem('token')
    const refreshToken = localStorage.getItem('refreshToken')
    if (token) {
      set({ token, refreshToken, isAuthenticated: true })
    }
  },
}))
