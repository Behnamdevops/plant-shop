import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getProduct } from '../api/products'
import type { Product } from '../types/product'

type ProductDetailsProps = {
  slug: string
}

function ProductDetails({ slug }: ProductDetailsProps) {
  const [product, setProduct] = useState<Product | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

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