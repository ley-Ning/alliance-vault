import type { AuthSession, AuthUser } from '../types'
import { requestApi } from './apiBase'

const SESSION_STORAGE_KEY = 'alliance-vault-auth-session-v1'

interface AuthPayload {
  user: AuthUser
  accessToken: string
  refreshToken: string
}

interface LoginPayload {
  username: string
  password: string
}

interface CreateTeamMemberPayload extends LoginPayload {
  displayName?: string
}

interface ChangePasswordPayload {
  currentPassword?: string
  newPassword: string
}

const requestJSON = async <T>(path: string, init?: RequestInit, accessToken?: string): Promise<T> => {
  const headers = new Headers(init?.headers)
  headers.set('Content-Type', 'application/json')
  if (accessToken) {
    headers.set('Authorization', `Bearer ${accessToken}`)
  }

  const response = await requestApi(path, {
    headers,
    ...init,
  })

  if (!response.ok) {
    let message = `请求失败（${response.status}）`
    try {
      const payload = (await response.json()) as { error?: string }
      if (payload?.error) {
        message = payload.error
      }
    } catch {
      // ignore
    }
    throw new Error(message)
  }

  return (await response.json()) as T
}

const normalizeSession = (payload: AuthPayload): AuthSession => ({
  user: payload.user,
  accessToken: payload.accessToken,
  refreshToken: payload.refreshToken,
})

export const getStoredSession = (): AuthSession | null => {
  try {
    const raw = localStorage.getItem(SESSION_STORAGE_KEY)
    if (!raw) {
      return null
    }
    const parsed = JSON.parse(raw) as AuthSession
    if (!parsed.accessToken || !parsed.refreshToken || !parsed.user?.id) {
      return null
    }
    return parsed
  } catch {
    return null
  }
}

export const setStoredSession = (session: AuthSession) => {
  localStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify(session))
}

export const clearStoredSession = () => {
  localStorage.removeItem(SESSION_STORAGE_KEY)
}

export const login = async (payload: LoginPayload): Promise<AuthSession> => {
  const result = await requestJSON<AuthPayload>('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
  const session = normalizeSession(result)
  setStoredSession(session)
  return session
}

export const changePassword = async (payload: ChangePasswordPayload): Promise<AuthSession> => {
  const current = getStoredSession()
  if (!current?.accessToken) {
    throw new Error('登录态已失效，请重新登录')
  }

  const result = await requestJSON<AuthPayload>('/api/v1/auth/change-password', {
    method: 'POST',
    body: JSON.stringify(payload),
  }, current.accessToken)
  const session = normalizeSession(result)
  setStoredSession(session)
  return session
}

export const createTeamMember = async (payload: CreateTeamMemberPayload): Promise<AuthUser> => {
  const current = getStoredSession()
  if (!current?.accessToken) {
    throw new Error('登录态已失效，请重新登录')
  }

  const result = await requestJSON<{ user: AuthUser }>('/api/v1/auth/team-members', {
    method: 'POST',
    body: JSON.stringify(payload),
  }, current.accessToken)
  return result.user
}

export const refreshSession = async (refreshToken: string): Promise<AuthSession> => {
  const result = await requestJSON<AuthPayload>('/api/v1/auth/refresh', {
    method: 'POST',
    body: JSON.stringify({ refreshToken }),
  })
  const session = normalizeSession(result)
  setStoredSession(session)
  return session
}

export const fetchMe = async (accessToken: string): Promise<AuthUser> => {
  const result = await requestJSON<{ user: AuthUser }>('/api/v1/auth/me', undefined, accessToken)
  return result.user
}

export const logout = async (session: AuthSession | null): Promise<void> => {
  if (!session) {
    clearStoredSession()
    return
  }

  try {
    await requestJSON<{ loggedOut: boolean }>('/api/v1/auth/logout', {
      method: 'POST',
      body: JSON.stringify({ refreshToken: session.refreshToken }),
    }, session.accessToken)
  } finally {
    clearStoredSession()
  }
}
