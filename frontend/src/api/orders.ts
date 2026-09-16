import type { CheckoutInput } from '../types/checkout'
import type { AdminOrderDetails, AdminOrderSummary, OrderDetails, OrderSummary, PaymentAttempt } from '../types/order'
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

// cancelOrder lets the authenticated customer cancel their own order,
// provided it belongs to them, its payment has not already succeeded, and
// its current status still allows cancellation (e.g. not already shipped/
// delivered/cancelled). Inventory is restored server-side.
export async function cancelOrder(id: number): Promise<OrderDetails> {
  const response = await fetch(`/api/v1/orders/${id}/cancel`, {
    method: 'POST',
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

// getAdminOrderPayments returns every ZarinPal payment attempt for order
// id, newest first, for admin inspection (e.g. showing a ref_id).
export async function getAdminOrderPayments(id: number): Promise<PaymentAttempt[]> {
  const response = await fetch(`/api/v1/admin/orders/${id}/payments`, {
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
