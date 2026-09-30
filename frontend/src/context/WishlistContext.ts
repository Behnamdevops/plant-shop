import { createContext, useContext } from 'react'
import type { Product } from '../types/product'
export const WishlistContext = createContext<{
  products: Product[]
  loading: boolean
  error: string
  toggle: (p: Product) => Promise<void>
} | null>(null)
export function useWishlist() {
  const context = useContext(WishlistContext)
  if (!context) throw new Error('WishlistProvider missing')
  return context
}
