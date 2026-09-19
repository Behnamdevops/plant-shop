import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { 
  createArticle, 
  getAdminArticleById, 
  getAdminArticleCategories, 
  updateArticle, 
  uploadArticleImage,
  type ArticleCategory,
  type ArticleInput 
} from '../../api/articles'

export default function AdminArticleFormPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const isEdit = Boolean(id)

  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [categories, setCategories] = useState<ArticleCategory[]>([])

  const [formData, setFormData] = useState<ArticleInput>({
    title: '',
    slug: '',
    excerpt: '',
    content: '',
    cover_image_url: null,
    category_id: undefined,
    status: 'draft',
    seo_title: null,
    seo_description: null
  })

  const [imageUploading, setImageUploading] = useState(false)

  useEffect(() => {
    getAdminArticleCategories()
      .then(setCategories)
      .catch(console.error)

    if (isEdit && id) {
      getAdminArticleById(parseInt(id, 10))
        .then((article) => {
          setFormData({
            title: article.title,
            slug: article.slug,
            excerpt: article.excerpt,
            content: article.content,
            cover_image_url: article.cover_image_url,
            category_id: article.category_id ?? undefined,
            status: article.status,
            seo_title: article.seo_title,
            seo_description: article.seo_description
          })
        })
        .catch((err) => {
          setError(err instanceof Error ? err.message : 'خطا در دریافت مقاله')
        })
    }
  }, [id, isEdit])

  const handleChange = (
    e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>
  ) => {
    const { name, value } = e.target
    setFormData(prev => ({ ...prev, [name]: value }))
  }

  const handleImageUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    setImageUploading(true)
    try {
      const result = await uploadArticleImage(file)
      setFormData(prev => ({ ...prev, cover_image_url: result.url }))
    } catch (err) {
      alert(err instanceof Error ? err.message : 'خطا در آپلود تصویر')
    } finally {
      setImageUploading(false)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    setError(null)

    try {
      const data: ArticleInput = {
        ...formData,
        category_id: formData.category_id ?? null,
        seo_title: formData.seo_title || null,
        seo_description: formData.seo_description || null
      }

      if (isEdit && id) {
        await updateArticle(parseInt(id, 10), data)
      } else {
        await createArticle(data)
      }
      navigate('/admin/articles')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'خطا در ذخیره مقاله')
    } finally {
      setSaving(false)
    }
  }

  const generateSlug = (title: string) => {
    return title
      .toLowerCase()
      .replace(/[\s\u0600-\u06FF]+/g, '-')
      .replace(/[^a-z0-9\u0600-\u06FF-]/g, '')
      .replace(/-+/g, '-')
      .replace(/^-|-$/g, '')
  }

  const handleTitleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const title = e.target.value
    setFormData(prev => ({
      ...prev,
      title,
      slug: prev.slug || generateSlug(title)
    }))
  }

  return (
    <div className="admin-page">
      <div className="admin-header">
        <h1>{isEdit ? 'ویرایش مقاله' : 'مقاله جدید'}</h1>
      </div>

      {error && <div className="error-message">{error}</div>}

      <form onSubmit={handleSubmit} className="admin-form">
        <div className="form-group">
          <label htmlFor="title">عنوان مقاله *</label>
          <input
            type="text"
            id="title"
            name="title"
            value={formData.title}
            onChange={handleTitleChange}
            required
            className="form-input"
          />
        </div>

        <div className="form-group">
          <label htmlFor="slug">اسلاگ *</label>
          <input
            type="text"
            id="slug"
            name="slug"
            value={formData.slug}
            onChange={handleChange}
            required
            className="form-input"
            placeholder="article-slug"
          />
        </div>

        <div className="form-group">
          <label htmlFor="excerpt">خلاصه</label>
          <textarea
            id="excerpt"
            name="excerpt"
            value={formData.excerpt}
            onChange={handleChange}
            className="form-input"
            rows={3}
            placeholder="خلاصه‌ای کوتاه از مقاله..."
          />
        </div>

        <div className="form-group">
          <label htmlFor="category_id">دسته‌بندی</label>
          <select
            id="category_id"
            name="category_id"
            value={formData.category_id ?? ''}
            onChange={handleChange}
            className="form-input"
          >
            <option value="">انتخاب دسته‌بندی</option>
            {categories.map(cat => (
              <option key={cat.id} value={cat.id}>{cat.name}</option>
            ))}
          </select>
        </div>

        <div className="form-group">
          <label htmlFor="cover_image_url">تصویر شاخص</label>
          <input
            type="file"
            id="cover_image"
            accept="image/jpeg,image/png,image/webp"
            onChange={handleImageUpload}
            disabled={imageUploading}
            className="form-input"
          />
          {imageUploading && <span>در حال آپلود...</span>}
          {formData.cover_image_url && (
            <div className="image-preview">
              <img src={formData.cover_image_url} alt="Cover" />
              <button
                type="button"
                onClick={() => setFormData(prev => ({ ...prev, cover_image_url: null }))}
                className="btn btn-sm btn-danger"
              >
                حذف
              </button>
            </div>
          )}
        </div>

        <div className="form-group">
          <label htmlFor="content">محتوای مقاله (Markdown) *</label>
          <textarea
            id="content"
            name="content"
            value={formData.content}
            onChange={handleChange}
            required
            className="form-input markdown-input"
            rows={15}
            placeholder="متن مقاله را به فرمت Markdown بنویسید..."
          />
          <small className="form-help">
            می‌توانید از قالب Markdown استفاده کنید: **bold**, *italic*, # heading, - list, etc.
          </small>
        </div>

        <div className="form-group">
          <label htmlFor="status">وضعیت</label>
          <select
            id="status"
            name="status"
            value={formData.status}
            onChange={handleChange}
            className="form-input"
          >
            <option value="draft">پیش‌نویس</option>
            <option value="published">منتشر شده</option>
          </select>
        </div>

        <div className="form-group">
          <label htmlFor="seo_title">عنوان سئو</label>
          <input
            type="text"
            id="seo_title"
            name="seo_title"
            value={formData.seo_title ?? ''}
            onChange={handleChange}
            className="form-input"
            placeholder="عنوان برای موتورهای جستجو (حداکثر ۷۰ کاراکتر)"
            maxLength={70}
          />
        </div>

        <div className="form-group">
          <label htmlFor="seo_description">توضیحات سئو</label>
          <textarea
            id="seo_description"
            name="seo_description"
            value={formData.seo_description ?? ''}
            onChange={handleChange}
            className="form-input"
            rows={2}
            placeholder="توضیحات برای موتورهای جستجو (حداکثر ۱۶۰ کاراکتر)"
            maxLength={160}
          />
        </div>

        <div className="form-actions">
          <button
            type="submit"
            disabled={saving}
            className="btn btn-primary"
          >
            {saving ? 'در حال ذخیره...' : isEdit ? 'ذخیره تغییرات' : 'ایجاد مقاله'}
          </button>
          <button
            type="button"
            onClick={() => navigate('/admin/articles')}
            className="btn btn-secondary"
          >
            انصراف
          </button>
        </div>
      </form>
    </div>
  )
}