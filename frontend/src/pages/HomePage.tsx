import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getProducts } from '../api/products'
import type { Product } from '../types/product'
import { storeConfig } from '../config'
import { formatToman } from '../lib/format'

export default function HomePage() {
  const [products, setProducts] = useState<Product[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    getProducts()
      .then(setProducts)
      .catch(() => setError('مشکلی در بارگذاری محصولات پیش آمد'))
      .finally(() => setLoading(false))
  }, [])

  if (loading) {
    return (
      <main>
        <p className="state-message">در حال بارگذاری محصولات...</p>
      </main>
    )
  }

  if (error) {
    return (
      <main>
        <p className="alert alert-error" role="alert">
          {error}
        </p>
      </main>
    )
  }

  return (
    <main>
      <div className="page-header">
        <h1>{storeConfig.name}</h1>
        <p className="page-subtitle">گیاهان سالم و تازه، تا در خانه شما.</p>
      </div>

      {products.length === 0 ? (
        <p className="empty-state">در حال حاضر محصولی موجود نیست.</p>
      ) : (
        <div className="product-grid">
          {products.map((product) => (
            <article key={product.id} className="card product-card">
              <div className="product-card__media">
                {product.image_url ? (
                  <img src={product.image_url} alt={product.name} />
                ) : (
                  <span className="product-card__media-placeholder" aria-hidden="true">
                    🌱
                  </span>
                )}
              </div>

              <div className="product-card__body">
                <h2>
                  <Link to={`/products/${product.slug}`}>{product.name}</Link>
                </h2>

                <p className="product-card__desc">{product.description}</p>

                <div className="product-card__footer">
                  <span className="price">{formatToman(product.price)}</span>
                  {product.stock > 0 ? (
                    <span className="badge badge-in-stock">موجود</span>
                  ) : (
                    <span className="badge badge-out-of-stock">ناموجود</span>
                  )}
                </div>

                <Link to={`/products/${product.slug}`} className="btn btn-secondary btn-block">
                  مشاهده جزئیات
                </Link>
              </div>
            </article>
          ))}
        </div>
      )}
    </main>
  )
}