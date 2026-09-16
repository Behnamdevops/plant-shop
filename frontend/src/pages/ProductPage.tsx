import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getProduct } from '../api/products'
import { addCartItem } from '../api/cart'
import type { Product } from '../types/product'
import { useAuth } from '../hooks/useAuth'
import { formatToman } from '../lib/format'

type ProductDetailsProps = {
  slug: string
}

function ProductDetails({ slug }: ProductDetailsProps) {
  const { user, loading: authLoading } = useAuth()
  const [product, setProduct] = useState<Product | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [cartMessage, setCartMessage] = useState('')
  const [cartError, setCartError] = useState('')
  const [addingToCart, setAddingToCart] = useState(false)

  useEffect(() => {
    let ignore = false

    getProduct(slug)
      .then((data) => {
        if (!ignore) {
          setProduct(data)
        }
      })
      .catch(() => {
        if (!ignore) {
          setError('مشکلی در بارگذاری محصول پیش آمد')
        }
      })
      .finally(() => {
        if (!ignore) {
          setLoading(false)
        }
      })

    return () => {
      ignore = true
    }
  }, [slug])

  if (loading) {
    return (
      <main>
        <p className="state-message">در حال بارگذاری محصول...</p>
      </main>
    )
  }

  if (error || !product) {
    return (
      <main>
        <p className="alert alert-error" role="alert">
          {error || 'محصول یافت نشد'}
        </p>
        <Link to="/" className="back-link">
          → بازگشت به محصولات
        </Link>
      </main>
    )
  }

  const handleAddToCart = async () => {
    setCartError('')
    setCartMessage('')
    setAddingToCart(true)

    try {
      await addCartItem(product.id, 1)
      setCartMessage('به سبد خرید اضافه شد')
    } catch (err) {
      setCartError(err instanceof Error ? err.message : 'مشکلی در افزودن به سبد خرید پیش آمد')
    } finally {
      setAddingToCart(false)
    }
  }

  return (
    <main>
      <Link to="/" className="back-link">
        → بازگشت
      </Link>

      <div className="product-detail">
        <div className="product-detail__media">
          {product.image_url ? (
            <img src={product.image_url} alt={product.name} />
          ) : (
            <span className="product-detail__media-placeholder" aria-hidden="true">
              🌱
            </span>
          )}
        </div>

        <div className="product-detail__info">
          <h1>{product.name}</h1>

          <div className="product-detail__price-row">
            <span className="price">{formatToman(product.price)}</span>
            {product.stock > 0 ? (
              <span className="badge badge-in-stock">موجود ({product.stock} عدد)</span>
            ) : (
              <span className="badge badge-out-of-stock">ناموجود</span>
            )}
          </div>

          <p className="product-detail__description">{product.description}</p>

          {!authLoading && (
            user ? (
              <>
                <button
                  type="button"
                  className="btn btn-primary"
                  onClick={handleAddToCart}
                  disabled={product.stock <= 0 || addingToCart}
                >
                  {product.stock <= 0
                    ? 'ناموجود'
                    : addingToCart
                      ? 'در حال افزودن...'
                      : 'افزودن به سبد خرید'}
                </button>
                {cartMessage && (
                  <p className="alert alert-success" role="status">
                    {cartMessage}
                  </p>
                )}
                {cartError && (
                  <p className="alert alert-error" role="alert">
                    {cartError}
                  </p>
                )}
              </>
            ) : (
              <p>
                برای افزودن این محصول به سبد خرید، <Link to="/login">وارد شوید</Link>.
              </p>
            )
          )}
        </div>
      </div>
    </main>
  )
}

export default function ProductPage() {
  const { slug } = useParams<{ slug: string }>()

  if (!slug) {
    return (
      <main>
        <p>محصول نامعتبر</p>
        <Link to="/">بازگشت به محصولات</Link>
      </main>
    )
  }

  return <ProductDetails key={slug} slug={slug} />
}