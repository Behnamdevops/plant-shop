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
          setError('محصول یافت نشد')
        } else {
          setError(err instanceof Error ? err.message : 'مشکلی در بارگذاری محصول پیش آمد')
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
          شناسه محصول نامعتبر است
        </p>
        <Link to="/admin/products" className="back-link">
          → بازگشت به محصولات
        </Link>
      </main>
    )
  }

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
        <Link to="/admin/products" className="back-link">
          → بازگشت به محصولات
        </Link>
      </main>
    )
  }

  return (
    <main>
      <div className="form-card">
        <h1>ویرایش محصول</h1>
        <ProductForm initial={product} submitLabel="ذخیره تغییرات" onSubmit={handleSubmit} />
      </div>
    </main>
  )
}
