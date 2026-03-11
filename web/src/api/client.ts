const BASE = '/api/v1'

function getToken(): string | null {
  return localStorage.getItem('token')
}

async function tryRefreshToken(): Promise<boolean> {
  const refreshToken = localStorage.getItem('refreshToken')
  if (!refreshToken) return false
  try {
    const resp = await fetch(`${BASE}/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken }),
    })
    if (!resp.ok) return false
    const data = await resp.json()
    localStorage.setItem('token', data.access_token)
    localStorage.setItem('refreshToken', data.refresh_token)
    return true
  } catch {
    return false
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getToken()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...((options.headers as Record<string, string>) || {}),
  }
  if (token) headers['Authorization'] = `Bearer ${token}`

  let resp = await fetch(`${BASE}${path}`, { ...options, headers })
  if (resp.status === 401 && await tryRefreshToken()) {
    headers['Authorization'] = `Bearer ${getToken()}`
    resp = await fetch(`${BASE}${path}`, { ...options, headers })
  }
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({}))
    throw new Error(body?.error?.message || `HTTP ${resp.status}`)
  }
  return resp.json()
}

// SSE helper for app creation
export function createAppSSE(
  prompt: string,
  onEvent: (event: string, data: any) => void,
  onError: (err: Error) => void,
): AbortController {
  const controller = new AbortController()

  async function doFetch() {
    const token = getToken()
    const resp = await fetch(`${BASE}/apps`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify({ prompt }),
      signal: controller.signal,
    })

    if (resp.status === 401) {
      const refreshed = await tryRefreshToken()
      if (refreshed) {
        const newToken = getToken()
        const retryResp = await fetch(`${BASE}/apps`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            ...(newToken ? { Authorization: `Bearer ${newToken}` } : {}),
          },
          body: JSON.stringify({ prompt }),
          signal: controller.signal,
        })
        if (!retryResp.ok) throw new Error(`HTTP ${retryResp.status}`)
        return retryResp
      }
      throw new Error('HTTP 401')
    }
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
    return resp
  }

  doFetch()
    .then(async (resp) => {
      const reader = resp.body!.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })

        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        let currentEvent = 'message'
        for (const line of lines) {
          if (line.startsWith('event: ')) {
            currentEvent = line.slice(7).trim()
          } else if (line.startsWith('data: ')) {
            try {
              const data = JSON.parse(line.slice(6))
              onEvent(currentEvent, data)
            } catch { /* ignore parse errors */ }
          }
        }
      }
    })
    .catch((err) => {
      if (err.name !== 'AbortError') onError(err)
    })

  return controller
}

// Auth
export const authAPI = {
  login: (username: string, password: string) =>
    request<{ access_token: string; refresh_token: string; expires_in: number }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  refresh: (refreshToken: string) =>
    request<{ access_token: string; refresh_token: string; expires_in: number }>('/auth/refresh', {
      method: 'POST',
      body: JSON.stringify({ refresh_token: refreshToken }),
    }),
}

// Setup
export const setupAPI = {
  status: () => request<{ completed: boolean; step: number }>('/setup/status'),
  step1: (username: string, password: string) =>
    request<{ ok: boolean }>('/setup/step/1', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
}

// Apps
export interface AppInfo {
  id: string
  name: string
  slug: string
  status: string
  container_id: string
  image_tag: string
  domain: string
  port: number
  prompt: string
  created_at: string
  updated_at: string
}

export const appsAPI = {
  list: () => request<AppInfo[]>('/apps'),
  get: (id: string) => request<AppInfo>(`/apps/${id}`),
  delete: (id: string) => request<{ ok: boolean }>(`/apps/${id}`, { method: 'DELETE' }),
  start: (id: string) => request<{ ok: boolean }>(`/apps/${id}/start`, { method: 'POST' }),
  stop: (id: string) => request<{ ok: boolean }>(`/apps/${id}/stop`, { method: 'POST' }),
}

// System
export const systemAPI = {
  status: () => request<{ status: string; version: string }>('/system/status'),
}
