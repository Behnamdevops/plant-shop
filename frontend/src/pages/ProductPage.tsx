import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getProduct } from '../api/products'
import { addCartItem } from '../api/cart'
import type { Product } from '../types/product'
import { useAuth } from '../hooks/useAuth'

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
          setError('Could not load product')
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
        <p className="state-message">Loading product...</p>
      </main>
    )
  }

  if (error || !product) {
    return (
      <main>
        <p className="alert alert-error" role="alert">
          {error || 'Product not found'}
        </p>
        <Link to="/" className="back-link">
          ← Back to products
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
      setCartMessage('Added to cart')
    } catch (err) {
      setCartError(err instanceof Error ? err.message : 'Could not add to cart')
    } finally {
      setAddingToCart(false)
    }
  }

  return (
    <main>
      <Link to="/" className="back-link">
        ← Back
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
            <span className="price">{product.price}</span>
            {product.stock > 0 ? (
              <span className="badge badge-in-stock">In stock ({product.stock})</span>
            ) : (
              <span className="badge badge-out-of-stock">Out of stock</span>
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
                    ? 'Out of stock'
                    : addingToCart
                      ? 'Adding...'
                      : 'Add to cart'}
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
                <Link to="/login">Log in</Link> to add this product to your cart.
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
        <p>Invalid product</p>
        <Link to="/">Back to products</Link>
      </main>
    )
  }

  return <ProductDetails key={slug} slug={slug} />
}