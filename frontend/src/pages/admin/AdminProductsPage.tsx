import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { deleteProduct, getProducts } from '../../api/products'
import { ApiError } from '../../api/errors'
import type { Product } from '../../types/product'

export default function AdminProductsPage() {
  const [products, setProducts] = useState<Product[] | null>(null)
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
    let ignore = false

    getProducts()
      .then((data) => {
        if (!ignore) setProducts(data)
      })
      .catch((err) => {
        if (!ignore) setError(err instanceof Error ? err.message : 'Could not load products')
      })
      .finally(() => {
        if (!ignore) setLoading(false)
      })

    return () => {
      ignore = true
    }
  }, [reloadKey])

  const handleDelete = async (product: Product) => {
    const confirmed = window.confirm(`Delete "${product.name}"? This cannot be undone.`)
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
        setDeleteError(
          `"${product.name}" cannot be deleted because it is referenced by existing orders.`,
        )
      } else {
        setDeleteError(err instanceof Error ? err.message : 'Could not delete product')
      }
    } finally {
      setDeletingId(null)
    }
  }

  return (
    <main>
      <div className="page-header">
        <h1>Admin · Products</h1>
        <p className="page-subtitle">Manage the product catalog.</p>
      </div>

      <Link to="/admin/products/new" className="btn btn-primary">
        + New product
      </Link>

      {deleteError && (
        <p className="alert alert-error" role="alert">
          {deleteError}
        </p>
      )}

      {loading && <p className="state-message">Loading products...</p>}

      {!loading && error && (
        <>
          <p className="alert alert-error" role="alert">
            {error}
          </p>
          <button type="button" className="btn btn-secondary" onClick={reload}>
            Retry
          </button>
        </>
      )}

      {!loading && !error && products && products.length === 0 && (
        <p className="empty-state">No products yet.</p>
      )}

      {!loading && !error && products && products.length > 0 && (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Slug</th>
                <th>Price</th>
                <th>Stock</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {products.map((product) => (
                <tr key={product.id}>
                  <td>{product.name}</td>
                  <td>{product.slug}</td>
                  <td className="price">{product.price}</td>
                  <td>{product.stock}</td>
                  <td>
                    <div style={{ display: 'flex', gap: 8 }}>
                      <Link to={`/admin/products/${product.id}/edit`} className="btn btn-secondary btn-sm">
                        Edit
                      </Link>
                      <button
                        type="button"
                        className="btn btn-danger btn-sm"
                        onClick={() => handleDelete(product)}
                        disabled={deletingId === product.id}
                      >
                        {deletingId === product.id ? 'Deleting...' : 'Delete'}
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
