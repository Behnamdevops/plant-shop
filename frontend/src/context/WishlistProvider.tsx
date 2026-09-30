import { useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import type { Product } from '../types/product'
import { useAuth } from '../hooks/useAuth'
import { readGuestWishlist, saveGuestWishlist } from '../lib/wishlist'
import { throwApiError } from '../api/errors'
import { WishlistContext } from './WishlistContext'
export default function WishlistProvider({
  children,
}: {
  children: ReactNode
}) {
  const { user, loading: authLoading } = useAuth()
  const [products, setProducts] = useState<Product[]>([]),
    [loading, setLoading] = useState(true),
    [error, setError] = useState('')
  useEffect(() => {
    if (authLoading) return
    let active = true
    const load = async () => {
      try {
        if (user) {
          const response = await fetch('/api/v1/wishlist')
          if (!response.ok) await throwApiError(response)
          const value = (await response.json()) as Product[]
          if (active) setProducts(value)
        } else if (active) setProducts(readGuestWishlist())
        if (active) setError('')
      } catch (err) {
        if (active)
          setError(
            err instanceof Error
              ? err.message
              : 'بارگذاری علاقه‌مندی‌ها انجام نشد.',
          )
      } finally {
        if (active) setLoading(false)
      }
    }
    void load()
    window.addEventListener('storage', load)
    window.addEventListener('wishlist-changed', load)
    return () => {
      active = false
      window.removeEventListener('storage', load)
      window.removeEventListener('wishlist-changed', load)
    }
  }, [user, authLoading])
  const toggle = async (p: Product) => {
    const saved = products.some((x) => x.id === p.id)
    if (!saved && products.length >= 100)
      throw new Error('حداکثر ۱۰۰ محصول را می‌توانید ذخیره کنید.')
    if (user) {
      const response = await fetch(
        saved ? `/api/v1/wishlist/${p.id}` : '/api/v1/wishlist/merge',
        {
          method: saved ? 'DELETE' : 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: saved ? undefined : JSON.stringify({ product_ids: [p.id] }),
        },
      )
      if (!response.ok) await throwApiError(response)
    }
    if (user) {
      setProducts((current) =>
        saved
          ? current.filter((x) => x.id !== p.id)
          : current.some((x) => x.id === p.id)
            ? current
            : [...current, p],
      )
    } else {
      const current = readGuestWishlist()
      const next = saved
        ? current.filter((x) => x.id !== p.id)
        : current.some((x) => x.id === p.id)
          ? current
          : [...current, p]
      saveGuestWishlist(next)
      setProducts(next)
    }
  }
  return (
    <WishlistContext.Provider value={{ products, loading, error, toggle }}>
      {children}
    </WishlistContext.Provider>
  )
}
