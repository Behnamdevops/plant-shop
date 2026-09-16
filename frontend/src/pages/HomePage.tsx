import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getProducts } from '../api/products'
import type { Product } from '../types/product'

export default function HomePage() {
  const [products, setProducts] = useState<Product[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    getProducts()
      .then(setProducts)
      .catch(() => setError('Could not load products'))
      .finally(() => setLoading(false))
  }, [])

  if (loading) {
    return <p>Loading products...</p>
  }

  if (error) {
    return <p>{error}</p>
  }

  return (
    <main>
      <h1>Plant Shop</h1>

      {products.map((product) => (
        <article key={product.id}>
          <h2>
            <Link to={`/products/${product.slug}`}>
              {product.name}
            </Link>
          </h2>

          {product.image_url && (
            <img
              src={product.image_url}
              alt={product.name}
              width="200"
            />
          )}

          <p>{product.description}</p>
          <p>Price: {product.price}</p>
          <p>Stock: {product.stock}</p>
        </article>
      ))}
    </main>
  )
}