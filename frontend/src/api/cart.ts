import type { Cart, CartItem } from '../types/cart'

// AddedItem mirrors the backend's cart.Item (POST /cart/items response),
// which is the raw cart_items row without the joined product fields that
// GetCart's ItemView provides (name, slug, price, subtotal).
export type AddedItem = {
  id: number
  product_id: number
  quantity: number
  created_at: string
  updated_at: string
}

async function readErrorMessage(response: Response): Promise<string> {
  const text = await response.text()
  return text.trim() || `Request failed with status ${response.status}`
}

export async function getCart(): Promise<Cart> {
  const response = await fetch('/api/v1/cart', {
    credentials: 'include',
  })

  if (!response.ok) {
    throw new Error(await readErrorMessage(response))
  }

  return response.json()
}

export async function addCartItem(productId: number, quantity: number): Promise<AddedItem> {
  const response = await fetch('/api/v1/cart/items', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ product_id: productId, quantity }),
  })

  if (!response.ok) {
    throw new Error(await readErrorMessage(response))
  }

  return response.json()
}

export async function updateCartItem(itemId: number, quantity: number): Promise<CartItem> {
  const response = await fetch(`/api/v1/cart/items/${itemId}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ quantity }),
  })

  if (!response.ok) {
    throw new Error(await readErrorMessage(response))
  }

  return response.json()
}

export async function deleteCartItem(itemId: number): Promise<void> {
  const response = await fetch(`/api/v1/cart/items/${itemId}`, {
    method: 'DELETE',
    credentials: 'include',
  })

  if (!response.ok && response.status !== 401) {
    throw new Error(await readErrorMessage(response))
  }
}
