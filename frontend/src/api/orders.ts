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

// ReturnRequestInput is the input for creating a return request.
export type ReturnRequestInput = {
  reason: string
  customer_note?: string
}

// ReturnRequest is the return request data returned from the API.
export type ReturnRequest = {
  id: number
  order_id: number
  status: string
  reason: string
  customer_note?: string
  admin_note?: string
  requested_at: string
  reviewed_at?: string
  received_at?: string
  created_at: string
  updated_at: string
}

// createReturnRequest creates a new return request for the authenticated user's order.
export async function createReturnRequest(id: number, input: ReturnRequestInput): Promise<ReturnRequest> {
  const response = await fetch(`/api/v1/orders/${id}/return-request`, {
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

// getReturnRequest gets the return request for the authenticated user's order.
export async function getReturnRequest(id: number): Promise<ReturnRequest> {
  const response = await fetch(`/api/v1/orders/${id}/return-request`, {
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

// Admin return request types
export type AdminReturnRequest = {
  id: number
  order_id: number
  user_id: number
  status: string
  reason: string
  customer_note?: string
  admin_note?: string
  requested_at: string
  reviewed_at?: string
  received_at?: string
  created_at: string
  updated_at: string
}

// AdminRefund is the refund record returned by the refund action endpoints.
// amount and payment_status always come from the server; the admin never
// supplies them.
export type AdminRefund = {
  id: number
  order_id: number
  return_request_id?: number
  amount: number
  payment_method: string
  status: string
  provider_refund_id?: string
  requested_by_admin_id?: number
  last_error?: string
  created_at: string
  updated_at: string
  completed_at?: string
}

export async function getAdminReturnRequests(): Promise<AdminReturnRequest[]> {
  const response = await fetch('/api/v1/admin/returns', {
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function getAdminReturnRequest(id: number): Promise<AdminReturnRequest> {
  const response = await fetch(`/api/v1/admin/returns/${id}`, {
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

// The admin return workflow uses explicit action endpoints instead of a
// generic status-mutation endpoint. Each action only accepts an optional
// note — never a status value, refund amount, or admin id — so the client
// can never set an arbitrary status or influence how much money moves.
export type ReturnActionInput = {
  note?: string
}

export async function approveReturnRequest(id: number, input: ReturnActionInput = {}): Promise<AdminReturnRequest> {
  const response = await fetch(`/api/v1/admin/returns/${id}/approve`, {
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

export async function rejectReturnRequest(id: number, input: ReturnActionInput = {}): Promise<AdminReturnRequest> {
  const response = await fetch(`/api/v1/admin/returns/${id}/reject`, {
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

export async function markReturnRequestReceived(id: number, input: ReturnActionInput = {}): Promise<AdminReturnRequest> {
  const response = await fetch(`/api/v1/admin/returns/${id}/received`, {
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

// refundReturnRequest triggers the provider-based refund action for a
// return request that has already been marked "received". It never
// accepts an amount or provider id from the caller — those always come
// from the authoritative order/refund records on the server.
export async function refundReturnRequest(id: number): Promise<AdminRefund> {
  const response = await fetch(`/api/v1/admin/returns/${id}/refund`, {
    method: 'POST',
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

// confirmManualRefund is the explicit, separate admin confirmation required
// to complete a refund for orders paid by a manual (non-gateway) method.
// It never runs implicitly as part of any other action.
export async function confirmManualRefund(id: number): Promise<AdminRefund> {
  const response = await fetch(`/api/v1/admin/returns/${id}/refund-manual`, {
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
