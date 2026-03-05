import { create } from 'zustand'
import { appsAPI, type AppInfo } from '../api/client'

interface AppState {
  apps: AppInfo[]
  loading: boolean
  fetchApps: () => Promise<void>
  removeApp: (id: string) => void
  addApp: (app: AppInfo) => void
  updateApp: (id: string, updates: Partial<AppInfo>) => void
}

export const useAppStore = create<AppState>((set) => ({
  apps: [],
  loading: false,

  fetchApps: async () => {
    set({ loading: true })
    try {
      const apps = await appsAPI.list()
      set({ apps, loading: false })
    } catch {
      set({ loading: false })
    }
  },

  removeApp: (id) =>
    set((state) => ({ apps: state.apps.filter((a) => a.id !== id) })),

  addApp: (app) =>
    set((state) => ({ apps: [app, ...state.apps] })),

  updateApp: (id, updates) =>
    set((state) => ({
      apps: state.apps.map((a) => (a.id === id ? { ...a, ...updates } : a)),
    })),
}))
