import { createContext } from 'react'
import type { AuthCredentials, RegisterInput } from '../api/auth'
import type { User } from '../types/user'

export type AuthContextValue = {
  user: User | null
  loading: boolean
  login: (input: AuthCredentials) => Promise<void>
  register: (input: RegisterInput) => Promise<void>
  logout: () => Promise<void>
}

export const AuthContext = createContext<AuthContextValue | null>(null)
