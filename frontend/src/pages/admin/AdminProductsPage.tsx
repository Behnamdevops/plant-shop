import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { deleteProduct, getProducts } from '../../api/products'
import { getAdminCategories } from '../../api/categories'
import { ApiError } from '../../api/errors'
import type { Product } from '../../types/product'
import type { Category } from '../../types/category'
import { formatToman } from '../../lib/format'

// Admin product management shows the full catalog, so it requests the
// maximum page size instead of paginating — admins need to scan/manage all
// products, unlike the public storefront where large catalogs must be
// paginated server-side.
const ADMIN_PAGE_SIZE = 100

export default function AdminProductsPage() {
  const [products, setProducts] = useState<Product[] | null>(null)
  const [categories, setCategories] = useState<Category[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [deletingId, setDeletingId] = useState<number | null>(null)
  const [deleteError, setDeleteError] = useState('')
  const [reloadKey, setReloadKey] = useState(0)

  const reload = () => {
    setLoading(true)
    setError('')
    setReloadKey((key) => key + 1)
  }

  useEffect(() => {
    getAdminCategories()
      .then(setCategories)
      .catch(() => {
        /* Category names are a display enhancement only; leave the column
           blank if this fails rather than blocking the product list. */
      })
  }, [])

  useEffect(() => {
    let ignore = false

    getProducts({ page_size: ADMIN_PAGE_SIZE })
      .then((data) => {
        if (!ignore) setProducts(data.items)
      })
      .catch((err) => {
        if (!ignore) setError(err instanceof Error ? err.message : 'مشکلی در بارگذاری محصولات پیش آمد')
      })
      .finally(() => {
        if (!ignore) setLoading(false)
      })

    return () => {
      ignore = true
    }
  }, [reloadKey])

  const categoryName = (categoryId: number | null) => {
    if (categoryId === null) return '—'
    return categories.find((c) => c.id === categoryId)?.name ?? '—'
  }

  const handleDelete = async (product: Product) => {
    const confirmed = window.confirm(`محصول «${product.name}» حذف شود؟ این عمل قابل بازگشت نیست.`)
    if (!confirmed) {
      return
    }

    setDeleteError('')
    setDeletingId(product.id)

    try {
      await deleteProduct(product.id)
      setProducts((current) => (current ? current.filter((p) => p.id !== product.id) : current))
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setDeleteError(`محصول «${product.name}» به دلیل وجود سفارش‌های مرتبط قابل حذف نیست.`)
      } else {
        setDeleteError(err instanceof Error ? err.message : 'مشکلی در حذف محصول پیش آمد')
      }
    } finally {
      setDeletingId(null)
    }
  }

  return (
    <main>
      <div className="page-header">
        <h1>مدیریت · محصولات</h1>
        <p className="page-subtitle">مدیریت فهرست محصولات فروشگاه.</p>
      </div>

      <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
        <Link to="/admin/products/new" className="btn btn-primary">
          + محصول جدید
        </Link>
        <Link to="/admin/categories" className="btn btn-secondary">
          مدیریت دسته‌بندی‌ها
        </Link>
      </div>

      {deleteError && (
        <p className="alert alert-error" role="alert">
          {deleteError}
        </p>
      )}

      {loading && <p className="state-message">در حال بارگذاری محصولات...</p>}

      {!loading && error && (
        <>
          <p className="alert alert-error" role="alert">
            {error}
          </p>
          <button type="button" className="btn btn-secondary" onClick={reload}>
            تلاش دوباره
          </button>
        </>
      )}

      {!loading && !error && products && products.length === 0 && (
        <p className="empty-state">هنوز محصولی ثبت نشده است.</p>
      )}

      {!loading && !error && products && products.length > 0 && (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>نام</th>
                <th>Slug</th>
                <th>دسته‌بندی</th>
                <th>قیمت</th>
                <th>موجودی</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {products.map((product) => (
                <tr key={product.id}>
                  <td>{product.name}</td>
                  <td>{product.slug}</td>
                  <td>{categoryName(product.category_id)}</td>
                  <td className="price">{formatToman(product.price)}</td>
                  <td>{product.stock}</td>
                  <td>
                    <div style={{ display: 'flex', gap: 8 }}>
                      <Link to={`/admin/products/${product.id}/edit`} className="btn btn-secondary btn-sm">
                        ویرایش
                      </Link>
                      <button
                        type="button"
                        className="btn btn-danger btn-sm"
                        onClick={() => handleDelete(product)}
                        disabled={deletingId === product.id}
                      >
                        {deletingId === product.id ? 'در حال حذف...' : 'حذف'}
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </main>
  )
}
