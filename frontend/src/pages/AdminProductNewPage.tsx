import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { createProduct } from '../api/products'
import type { ProductInput } from '../api/products'

export default function AdminProductNewPage() {
  const navigate = useNavigate()

  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [description, setDescription] = useState('')
  const [price, setPrice] = useState('')
  const [stock, setStock] = useState('')
  const [imageUrl, setImageUrl] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setError('')

    const trimmedName = name.trim()
    const trimmedSlug = slug.trim()

    if (!trimmedName || !trimmedSlug) {
      setError('Name and slug are required')
      return
    }

    const priceValue = Number(price)
    if (price.trim() === '' || Number.isNaN(priceValue) || !Number.isInteger(priceValue)) {
      setError('Price must be a whole number')
      return
    }
    if (priceValue < 0) {
      setError('Price must be >= 0')
      return
    }

    const stockValue = Number(stock)
    if (stock.trim() === '' || Number.isNaN(stockValue) || !Number.isInteger(stockValue)) {
      setError('Stock must be a whole number')
      return
    }
    if (stockValue < 0) {
      setError('Stock must be >= 0')
      return
    }

    const input: ProductInput = {
      name: trimmedName,
      slug: trimmedSlug,
      description: description.trim(),
      price: priceValue,
      stock: stockValue,
      image_url: imageUrl.trim() || null,
    }

    setSubmitting(true)

    try {
      await createProduct(input)
      navigate('/admin/products')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not create product')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <main>
      <Link to="/admin/products" className="back-link">
        ← Back to products
      </Link>

      <div className="form-card admin-form-card">
        <h1>New product</h1>

        <form onSubmit={handleSubmit}>
          <div className="form-field">
            <label htmlFor="product-name">Name</label>
            <input
              id="product-name"
              type="text"
              value={name}
              onChange={(event) => setName(event.target.value)}
              disabled={submitting}
              required
            />
          </div>

          <div className="form-field">
            <label htmlFor="product-slug">Slug</label>
            <input
              id="product-slug"
              type="text"
              value={slug}
              onChange={(event) => setSlug(event.target.value)}
              disabled={submitting}
              required
            />
          </div>

          <div className="form-field">
            <label htmlFor="product-description">Description</label>
            <textarea
              id="product-description"
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              disabled={submitting}
              rows={4}
            />
          </div>

          <div className="form-field">
            <label htmlFor="product-price">Price</label>
            <input
              id="product-price"
              type="number"
              min={0}
              step={1}
              value={price}
              onChange={(event) => setPrice(event.target.value)}
              disabled={submitting}
              required
            />
          </div>

          <div className="form-field">
            <label htmlFor="product-stock">Stock</label>
            <input
              id="product-stock"
              type="number"
              min={0}
              step={1}
              value={stock}
              onChange={(event) => setStock(event.target.value)}
              disabled={submitting}
              required
            />
          </div>

          <div className="form-field">
            <label htmlFor="product-image-url">Image URL (optional)</label>
            <input
              id="product-image-url"
              type="text"
              value={imageUrl}
              onChange={(event) => setImageUrl(event.target.value)}
              disabled={submitting}
            />
          </div>

          {error && (
            <p className="alert alert-error" role="alert">
              {error}
            </p>
          )}

          <button type="submit" className="btn btn-primary btn-block" disabled={submitting}>
            {submitting ? 'Creating...' : 'Create product'}
          </button>
        </form>
      </div>
    </main>
  )
}
