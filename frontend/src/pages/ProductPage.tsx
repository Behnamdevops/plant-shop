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
    return <p>Loading product...</p>
  }

  if (error || !product) {
    return (
      <main>
        <p>{error || 'Product not found'}</p>
        <Link to="/">Back to products</Link>
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
      <Link to="/">← Back</Link>

      <h1>{product.name}</h1>

      {product.image_url && (
        <img
          src={product.image_url}
          alt={product.name}
          width="300"
        />
      )}

      <p>{product.description}</p>
      <p>Price: {product.price}</p>
      <p>Stock: {product.stock}</p>

      {!authLoading && (
        user ? (
          <>
            <button
              type="button"
              onClick={handleAddToCart}
              disabled={product.stock <= 0 || addingToCart}
            >
              {product.stock <= 0
                ? 'Out of stock'
                : addingToCart
                  ? 'Adding...'
                  : 'Add to cart'}
            </button>
            {cartMessage && <p role="status">{cartMessage}</p>}
            {cartError && <p role="alert">{cartError}</p>}
          </>
        ) : (
          <p>
            <Link to="/login">Log in</Link> to add this product to your cart.
          </p>
        )
      )}
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