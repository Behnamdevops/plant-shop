import { useCallback, useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import {
  getCurrentUser,
  login as apiLogin,
  logout as apiLogout,
  register as apiRegister,
} from '../api/auth'
import type { AuthCredentials, RegisterInput } from '../api/auth'
import type { User } from '../types/user'
import { AuthContext } from './AuthContext'

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    getCurrentUser()
      .then(setUser)
      .catch(() => setUser(null))
      .finally(() => setLoading(false))
  }, [])

  const login = useCallback(async (input: AuthCredentials) => {
    const loggedInUser = await apiLogin(input)
    setUser(loggedInUser)
  }, [])

  const register = useCallback(async (input: RegisterInput) => {
    const registeredUser = await apiRegister(input)
    setUser(registeredUser)
  }, [])

  const logout = useCallback(async () => {
    try {
      await apiLogout()
    } finally {
      setUser(null)
    }
  }, [])

  return (
    <AuthContext.Provider value={{ user, loading, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  )
}
