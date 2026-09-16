import { useState } from 'react'
import type { FormEvent } from 'react'
import type { ProductInput } from '../../api/products'
import type { Product } from '../../types/product'

type ProductFormProps = {
  initial?: Product
  submitLabel: string
  onSubmit: (input: ProductInput) => Promise<void>
}

export default function ProductForm({ initial, submitLabel, onSubmit }: ProductFormProps) {
  const [name, setName] = useState(initial?.name ?? '')
  const [slug, setSlug] = useState(initial?.slug ?? '')
  const [description, setDescription] = useState(initial?.description ?? '')
  const [price, setPrice] = useState(initial ? String(initial.price) : '')
  const [stock, setStock] = useState(initial ? String(initial.stock) : '')
  const [imageUrl, setImageUrl] = useState(initial?.image_url ?? '')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setError('')

    const trimmedName = name.trim()
    const trimmedSlug = slug.trim()
    const priceNum = Number(price)
    const stockNum = Number(stock)

    if (!trimmedName || !trimmedSlug) {
      setError('Name and slug are required')
      return
    }
    if (!Number.isFinite(priceNum) || priceNum < 0) {
      setError('Price must be a number >= 0')
      return
    }
    if (!Number.isInteger(stockNum) || stockNum < 0) {
      setError('Stock must be a whole number >= 0')
      return
    }

    setSubmitting(true)

    try {
      await onSubmit({
        name: trimmedName,
        slug: trimmedSlug,
        description: description.trim(),
        price: priceNum,
        stock: stockNum,
        image_url: imageUrl.trim() || null,
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not save product')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={handleSubmit}>
      <div className="form-field">
        <label htmlFor="product-name">Name</label>
        <input
          id="product-name"
          type="text"
          value={name}
          onChange={(event) => setName(event.target.value)}
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
          required
        />
      </div>

      <div className="form-field">
        <label htmlFor="product-description">Description</label>
        <input
          id="product-description"
          type="text"
          value={description}
          onChange={(event) => setDescription(event.target.value)}
        />
      </div>

      <div className="form-field">
        <label htmlFor="product-price">Price (minor units)</label>
        <input
          id="product-price"
          type="number"
          min={0}
          value={price}
          onChange={(event) => setPrice(event.target.value)}
          required
        />
      </div>

      <div className="form-field">
        <label htmlFor="product-stock">Stock</label>
        <input
          id="product-stock"
          type="number"
          min={0}
          value={stock}
          onChange={(event) => setStock(event.target.value)}
          required
        />
      </div>

      <div className="form-field">
        <label htmlFor="product-image-url">Image URL</label>
        <input
          id="product-image-url"
          type="text"
          value={imageUrl}
          onChange={(event) => setImageUrl(event.target.value)}
        />
      </div>

      {error && (
        <p className="alert alert-error" role="alert">
          {error}
        </p>
      )}

      <button type="submit" className="btn btn-primary btn-block" disabled={submitting}>
        {submitting ? 'Saving...' : submitLabel}
      </button>
    </form>
  )
}
