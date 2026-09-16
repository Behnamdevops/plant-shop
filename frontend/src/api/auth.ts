import type { User } from '../types/user'
import { throwApiError } from './errors'

export type AuthCredentials = {
  email: string
  password: string
}

export type RegisterInput = AuthCredentials & {
  name: string
}

export async function register(input: RegisterInput): Promise<User> {
  const response = await fetch('/api/v1/auth/register', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify(input),
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function login(input: AuthCredentials): Promise<User> {
  const response = await fetch('/api/v1/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify(input),
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function logout(): Promise<void> {
  const response = await fetch('/api/v1/auth/logout', {
    method: 'POST',
    credentials: 'include',
  })

  if (!response.ok && response.status !== 401) {
    return throwApiError(response)
  }
}

export async function getCurrentUser(): Promise<User | null> {
  const response = await fetch('/api/v1/me', {
    credentials: 'include',
  })

  if (response.status === 401) {
    return null
  }

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}
