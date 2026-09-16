import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { getAdminProduct, updateProduct } from '../../api/products'
import type { ProductInput } from '../../api/products'
import { ApiError } from '../../api/errors'
import type { Product } from '../../types/product'
import ProductForm from './ProductForm'

export default function AdminProductEditPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [product, setProduct] = useState<Product | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const productId = id ? Number(id) : NaN
  const validId = Number.isFinite(productId)

  useEffect(() => {
    if (!validId) {
      return
    }

    let ignore = false

    getAdminProduct(productId)
      .then((data) => {
        if (!ignore) setProduct(data)
      })
      .catch((err) => {
        if (ignore) return
        if (err instanceof ApiError && err.status === 404) {
          setError('Product not found')
        } else {
          setError(err instanceof Error ? err.message : 'Could not load product')
        }
      })
      .finally(() => {
        if (!ignore) setLoading(false)
      })

    return () => {
      ignore = true
    }
  }, [productId, validId])

  const handleSubmit = async (input: ProductInput) => {
    await updateProduct(productId, input)
    navigate('/admin/products')
  }

  if (!validId) {
    return (
      <main>
        <p className="alert alert-error" role="alert">
          Invalid product id
        </p>
        <Link to="/admin/products" className="back-link">
          ← Back to admin products
        </Link>
      </main>
    )
  }

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
        <Link to="/admin/products" className="back-link">
          ← Back to admin products
        </Link>
      </main>
    )
  }

  return (
    <main>
      <div className="form-card">
        <h1>Edit product</h1>
        <ProductForm initial={product} submitLabel="Save changes" onSubmit={handleSubmit} />
      </div>
    </main>
  )
}
