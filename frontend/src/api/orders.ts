import type { OrderDetails, OrderSummary } from '../types/order'

async function readErrorMessage(response: Response): Promise<string> {
  const text = await response.text()
  return text.trim() || `Request failed with status ${response.status}`
}

export async function createOrder(): Promise<OrderSummary> {
  const response = await fetch('/api/v1/orders', {
    method: 'POST',
    credentials: 'include',
  })

  if (!response.ok) {
    throw new Error(await readErrorMessage(response))
  }

  return response.json()
}

export async function getOrders(): Promise<OrderSummary[]> {
  const response = await fetch('/api/v1/orders', {
    credentials: 'include',
  })

  if (!response.ok) {
    throw new Error(await readErrorMessage(response))
  }

  return response.json()
}

export async function getOrder(id: number): Promise<OrderDetails> {
  const response = await fetch(`/api/v1/orders/${id}`, {
    credentials: 'include',
  })

  if (!response.ok) {
    throw new Error(await readErrorMessage(response))
  }

  return response.json()
}
