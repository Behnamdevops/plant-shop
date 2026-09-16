import type { CheckoutInput } from '../types/checkout'
import type { AdminOrderDetails, AdminOrderSummary, OrderDetails, OrderSummary } from '../types/order'
import { throwApiError } from './errors'

// createOrder submits a checkout request for the authenticated user's
// current cart. input carries only user-provided delivery/shipping
// details — the server always recalculates the item subtotal, shipping
// fee, and total itself, and the user id always comes from the session
// cookie, never from this body.
export async function createOrder(input: CheckoutInput): Promise<OrderSummary> {
  const response = await fetch('/api/v1/orders', {
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

export async function getOrders(): Promise<OrderSummary[]> {
  const response = await fetch('/api/v1/orders', {
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function getOrder(id: number): Promise<OrderDetails> {
  const response = await fetch(`/api/v1/orders/${id}`, {
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function getAdminOrders(): Promise<AdminOrderSummary[]> {
  const response = await fetch('/api/v1/admin/orders', {
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function getAdminOrder(id: number): Promise<AdminOrderDetails> {
  const response = await fetch(`/api/v1/admin/orders/${id}`, {
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function updateAdminOrderStatus(id: number, status: string): Promise<AdminOrderDetails> {
  const response = await fetch(`/api/v1/admin/orders/${id}/status`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ status }),
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}
