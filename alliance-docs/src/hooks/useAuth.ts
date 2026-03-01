import { useEffect, useState } from 'react'
import {
  changePassword,
  clearStoredSession,
  createTeamMember,
  fetchMe,
  getStoredSession,
  login,
  logout,
  refreshSession,
  setStoredSession,
} from '../lib/auth'
import type { AuthSession } from '../types'

export const useAuth = () => {
  const [session, setSession] = useState<AuthSession | null>(() => getStoredSession())
  const [booting, setBooting] = useState(true)

  useEffect(() => {
    let cancelled = false

    const bootstrap = async () => {
      const current = getStoredSession()
      if (!current) {
        if (!cancelled) {
          setBooting(false)
        }
        return
      }

      try {
        const refreshed = await refreshSession(current.refreshToken)
        const me = await fetchMe(refreshed.accessToken)
        const normalized: AuthSession = { ...refreshed, user: me }
        if (!cancelled) {
          setStoredSession(normalized)
          setSession(normalized)
        }
      } catch {
        if (!cancelled) {
          clearStoredSession()
          setSession(null)
        }
      } finally {
        if (!cancelled) {
          setBooting(false)
        }
      }
    }

    bootstrap()
    return () => {
      cancelled = true
    }
  }, [])

  const handleLogin = async (username: string, password: string) => {
    const next = await login({ username, password })
    setSession(next)
  }

  const handleChangePassword = async (newPassword: string, currentPassword?: string) => {
    const next = await changePassword({ currentPassword, newPassword })
    setSession(next)
  }

  const handleCreateTeamMember = async (username: string, password: string, displayName?: string) => {
    await createTeamMember({ username, password, displayName })
  }

  const handleLogout = async () => {
    const current = getStoredSession()
    await logout(current)
    setSession(null)
  }

  return {
    session,
    booting,
    login: handleLogin,
    changePassword: handleChangePassword,
    createTeamMember: handleCreateTeamMember,
    logout: handleLogout,
  }
}
