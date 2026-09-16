import type { OrderDetails, OrderSummary } from '../types/order'
import { throwApiError } from './errors'

export async function createOrder(): Promise<OrderSummary> {
  const response = await fetch('/api/v1/orders', {
    method: 'POST',
    credentials: 'include',
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
