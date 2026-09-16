import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getProducts } from '../api/products'
import type { Product } from '../types/product'

export default function AdminProductsPage() {
  const [products, setProducts] = useState<Product[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    getProducts()
      .then(setProducts)
      .catch(() => setError('Could not load products'))
      .finally(() => setLoading(false))
  }, [])

  return (
    <main>
      <div className="page-header admin-page-header">
        <div>
          <h1>Manage products</h1>
          <p className="page-subtitle">Create, edit, and review the product catalog.</p>
        </div>
        <Link to="/admin/products/new" className="btn btn-primary">
          + New product
        </Link>
      </div>

      {loading ? (
        <p className="state-message">Loading products...</p>
      ) : error ? (
        <p className="alert alert-error" role="alert">
          {error}
        </p>
      ) : products.length === 0 ? (
        <p className="empty-state">No products yet. Create the first one to get started.</p>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Image</th>
                <th>ID</th>
                <th>Name</th>
                <th>Slug</th>
                <th>Price</th>
                <th>Stock</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {products.map((product) => (
                <tr key={product.id}>
                  <td>
                    {product.image_url ? (
                      <img
                        src={product.image_url}
                        alt={product.name}
                        className="admin-thumb"
                      />
                    ) : (
                      <span className="admin-thumb admin-thumb--placeholder" aria-hidden="true">
                        🌱
                      </span>
                    )}
                  </td>
                  <td>{product.id}</td>
                  <td>{product.name}</td>
                  <td>{product.slug}</td>
                  <td className="price">{product.price}</td>
                  <td>{product.stock}</td>
                  <td>
                    <Link
                      to={`/admin/products/${product.id}/edit`}
                      className="btn btn-secondary btn-sm"
                    >
                      Edit
                    </Link>
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
