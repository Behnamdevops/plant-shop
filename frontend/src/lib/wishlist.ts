import type { Product } from '../types/product'
import { throwApiError } from '../api/errors'
const KEY = 'plantshop.wishlist.v1'
export function readGuestWishlist(): Product[] {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(KEY) || '[]')
    return Array.isArray(value)
      ? value
          .filter(
            (x): x is Product =>
              !!x &&
              Number.isSafeInteger(x.id) &&
              x.id > 0 &&
              typeof x.slug === 'string',
          )
          .slice(0, 100)
      : []
  } catch {
    return []
  }
}
export function saveGuestWishlist(products: Product[]) {
  localStorage.setItem(KEY, JSON.stringify(products))
  window.dispatchEvent(new Event('wishlist-changed'))
}
export async function mergeGuestWishlist() {
  const items = readGuestWishlist()
  if (!items.length) return
  const response = await fetch('/api/v1/wishlist/merge', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ product_ids: items.map((p) => p.id) }),
  })
  if (!response.ok) await throwApiError(response)
  saveGuestWishlist(
    readGuestWishlist().filter((p) => !items.some((old) => old.id === p.id)),
  )
}
