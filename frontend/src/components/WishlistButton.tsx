import { useState } from 'react'
import type { Product } from '../types/product'
import { useWishlist } from '../context/WishlistContext'
export default function WishlistButton({ product }: { product: Product }) {
  const { products, toggle, loading } = useWishlist()
  const saved = products.some((p) => p.id === product.id)
  const [busy, setBusy] = useState(false),
    [error, setError] = useState('')
  return (
    <>
      <button
        type="button"
        className="wishlist-button btn btn-secondary btn-sm"
        aria-label={saved ? 'حذف از علاقه‌مندی‌ها' : 'افزودن به علاقه‌مندی‌ها'}
        aria-pressed={saved}
        disabled={busy || loading}
        onClick={async () => {
          setBusy(true)
          setError('')
          try {
            await toggle(product)
          } catch (err) {
            setError(err instanceof Error ? err.message : 'ذخیره انجام نشد.')
          } finally {
            setBusy(false)
          }
        }}
      >
        {saved ? '♥' : '♡'}
      </button>
      {error && <span role="alert">{error}</span>}
    </>
  )
}
