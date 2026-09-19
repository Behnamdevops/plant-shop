import { throwApiError } from './errors'

export interface Profile {
  id: number
  name: string
  email: string
  phone: string | null
  role: string
  created_at: string
  updated_at: string
}

export interface ProfileUpdateInput {
  name: string
  phone: string
}

export async function getProfile(): Promise<Profile> {
  const res = await fetch('/api/v1/account/profile', {
    credentials: 'include',
  })
  if (!res.ok) {
    await throwApiError(res)
  }
  return res.json()
}

export async function updateProfile(input: ProfileUpdateInput): Promise<Profile> {
  const res = await fetch('/api/v1/account/profile', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify(input),
  })
  if (!res.ok) {
    await throwApiError(res)
  }
  return res.json()
}