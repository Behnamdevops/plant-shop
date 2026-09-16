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
      setError('نام و Slug الزامی است')
      return
    }
    if (!Number.isFinite(priceNum) || priceNum < 0) {
      setError('قیمت باید عددی بزرگتر یا مساوی صفر باشد')
      return
    }
    if (!Number.isInteger(stockNum) || stockNum < 0) {
      setError('موجودی باید عددی صحیح و بزرگتر یا مساوی صفر باشد')
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
      setError(err instanceof Error ? err.message : 'مشکلی در ذخیره محصول پیش آمد')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={handleSubmit}>
      <div className="form-field">
        <label htmlFor="product-name">نام</label>
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
        <label htmlFor="product-description">توضیحات</label>
        <input
          id="product-description"
          type="text"
          value={description}
          onChange={(event) => setDescription(event.target.value)}
        />
      </div>

      <div className="form-field">
        <label htmlFor="product-price">قیمت (ریال)</label>
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
        <label htmlFor="product-stock">موجودی</label>
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
        <label htmlFor="product-image-url">آدرس تصویر</label>
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
        {submitting ? 'در حال ذخیره...' : submitLabel}
      </button>
    </form>
  )
}
