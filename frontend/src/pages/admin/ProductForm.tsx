import { useEffect, useRef, useState } from 'react'
import type { ChangeEvent, FormEvent } from 'react'
import type { ProductInput } from '../../api/products'
import { uploadProductImage } from '../../api/products'
import type { Product } from '../../types/product'
import type { Category } from '../../types/category'
import { getAdminCategories } from '../../api/categories'
import { ApiError } from '../../api/errors'
import ProductImage from '../../components/ProductImage'

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
  const [categoryId, setCategoryId] = useState(initial?.category_id != null ? String(initial.category_id) : '')
  const [categories, setCategories] = useState<Category[]>([])
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [uploadError, setUploadError] = useState('')
  const fileInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    getAdminCategories()
      .then(setCategories)
      .catch(() => {
        /* Category dropdown is optional; leave it empty (uncategorized-only)
           if this fails rather than blocking the product form. */
      })
  }, [])

  const handleFileChange = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    // Reset the input value so selecting the same file again after an
    // error still fires onChange.
    event.target.value = ''
    if (!file) {
      return
    }

    setUploadError('')
    setUploading(true)

    try {
      const result = await uploadProductImage(file)
      setImageUrl(result.url)
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 401) {
          setUploadError('نشست شما منقضی شده است. دوباره وارد شوید.')
        } else if (err.status === 413) {
          setUploadError('حجم فایل بیشتر از حد مجاز است (حداکثر ۵ مگابایت)')
        } else if (err.status === 400) {
          setUploadError('فرمت فایل پشتیبانی نمی‌شود. فقط JPEG، PNG و WebP مجاز است')
        } else {
          setUploadError(err.message)
        }
      } else {
        setUploadError(err instanceof Error ? err.message : 'مشکلی در بارگذاری تصویر پیش آمد')
      }
    } finally {
      setUploading(false)
    }
  }

  const handleClearImage = () => {
    setImageUrl('')
    setUploadError('')
    if (fileInputRef.current) {
      fileInputRef.current.value = ''
    }
  }

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
        category_id: categoryId ? Number(categoryId) : null,
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
        <label htmlFor="product-image-file">تصویر محصول</label>

        <div className="product-image-field__preview">
          <ProductImage
            src={imageUrl || null}
            alt={name || 'پیش‌نمایش تصویر محصول'}
            className="product-image-field__preview-img"
            placeholderClassName="product-image-field__preview-placeholder"
          />
        </div>

        <input
          id="product-image-file"
          ref={fileInputRef}
          type="file"
          accept="image/jpeg,image/png,image/webp"
          onChange={handleFileChange}
          disabled={uploading || submitting}
        />

        {uploading && <p className="state-message state-message--inline">در حال بارگذاری...</p>}

        {uploadError && (
          <p className="alert alert-error" role="alert">
            {uploadError}
          </p>
        )}

        {imageUrl && !uploading && (
          <button
            type="button"
            className="btn btn-secondary btn-block"
            onClick={handleClearImage}
            disabled={submitting}
          >
            حذف تصویر / بدون تصویر
          </button>
        )}

        <p className="form-field__hint">
          فرمت‌های مجاز: JPEG، PNG، WebP — حداکثر ۵ مگابایت. می‌توانید به‌جای بارگذاری، آدرس تصویر خارجی را مستقیماً وارد کنید.
        </p>
        <input
          id="product-image-url"
          type="text"
          placeholder="یا آدرس تصویر را وارد کنید"
          value={imageUrl}
          onChange={(event) => setImageUrl(event.target.value)}
          disabled={uploading}
        />
      </div>

      <div className="form-field">
        <label htmlFor="product-category">دسته‌بندی</label>
        <select
          id="product-category"
          value={categoryId}
          onChange={(event) => setCategoryId(event.target.value)}
        >
          <option value="">بدون دسته‌بندی</option>
          {categories.map((category) => (
            <option key={category.id} value={category.id}>
              {category.name}
            </option>
          ))}
        </select>
      </div>

      {error && (
        <p className="alert alert-error" role="alert">
          {error}
        </p>
      )}

      <button type="submit" className="btn btn-primary btn-block" disabled={submitting || uploading}>
        {submitting ? 'در حال ذخیره...' : submitLabel}
      </button>
    </form>
  )
}
