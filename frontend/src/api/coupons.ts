import type { AdminCoupon, CouponInput, CouponPreview } from '../types/coupon'
import { throwApiError } from './errors'

// previewCoupon loads the authenticated user's current cart server-side
// and validates/calculates the supplied coupon code against it. This is
// advisory only — the server never persists a redemption here, and order
// creation always revalidates/recalculates everything from scratch. Never
// trust this response as authoritative when submitting the final order.
export async function previewCoupon(code: string): Promise<CouponPreview> {
  const response = await fetch('/api/v1/coupons/preview', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ code }),
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function getAdminCoupons(): Promise<AdminCoupon[]> {
  const response = await fetch('/api/v1/admin/coupons', {
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function getAdminCoupon(id: number): Promise<AdminCoupon> {
  const response = await fetch(`/api/v1/admin/coupons/${id}`, {
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function createCoupon(input: CouponInput): Promise<AdminCoupon> {
  const response = await fetch('/api/v1/admin/coupons', {
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

export async function updateCoupon(id: number, input: Partial<CouponInput>): Promise<AdminCoupon> {
  const response = await fetch(`/api/v1/admin/coupons/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify(input),
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}
