import { throwApiError } from './errors'

export interface Address {
  id: number
  label: string
  recipient_name: string
  phone: string
  address_line1: string
  address_line2: string | null
  city: string
  postal_code: string
  country: string
  is_default: boolean
  created_at: string
  updated_at: string
}

export interface AddressCreateInput {
  label: string
  recipient_name: string
  phone: string
  address_line1: string
  address_line2: string
  city: string
  postal_code: string
  country: string
  is_default: boolean
}

export type AddressUpdateInput = AddressCreateInput

export async function listAddresses(): Promise<Address[]> {
  const res = await fetch('/api/v1/account/addresses', {
    credentials: 'include',
  })
  if (!res.ok) {
    await throwApiError(res)
  }
  return res.json()
}

export async function createAddress(input: AddressCreateInput): Promise<Address> {
  const res = await fetch('/api/v1/account/addresses', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify(input),
  })
  if (!res.ok) {
    await throwApiError(res)
  }
  return res.json()
}

export async function updateAddress(id: number, input: AddressUpdateInput): Promise<Address> {
  const res = await fetch(`/api/v1/account/addresses/${id}`, {
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

export async function deleteAddress(id: number): Promise<void> {
  const res = await fetch(`/api/v1/account/addresses/${id}`, {
    method: 'DELETE',
    credentials: 'include',
  })
  if (!res.ok) {
    await throwApiError(res)
  }
}