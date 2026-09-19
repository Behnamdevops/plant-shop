import { useEffect, useState } from 'react'
import { 
  createArticleCategory, 
  deleteArticleCategory, 
  getAdminArticleCategories, 
  updateArticleCategory,
  type ArticleCategory 
} from '../../api/articles'

export default function AdminArticleCategoriesPage() {
  const [categories, setCategories] = useState<ArticleCategory[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [showForm, setShowForm] = useState(false)
  
  const [formData, setFormData] = useState({ name: '', slug: '' })
  const [saving, setSaving] = useState(false)

  const loadCategories = () => {
    getAdminArticleCategories()
      .then(setCategories)
      .catch((err) => {
        setError(err instanceof Error ? err.message : 'خطا در دریافت دسته‌بندی‌ها')
      })
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    loadCategories()
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    setError(null)

    try {
      if (editingId) {
        await updateArticleCategory(editingId, formData)
      } else {
        await createArticleCategory(formData)
      }
      setFormData({ name: '', slug: '' })
      setShowForm(false)
      setEditingId(null)
      loadCategories()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'خطا در ذخیره دسته‌بندی')
    } finally {
      setSaving(false)
    }
  }

  const handleEdit = (cat: ArticleCategory) => {
    setFormData({ name: cat.name, slug: cat.slug })
    setEditingId(cat.id)
    setShowForm(true)
  }

  const handleDelete = async (id: number) => {
    if (!confirm('آیا مطمئن هستید که می‌خواهید این دسته‌بندی را حذف کنید؟')) {
      return
    }

    try {
      await deleteArticleCategory(id)
      setCategories(categories.filter(c => c.id !== id))
    } catch (err) {
      alert(err instanceof Error ? err.message : 'خطا در حذف دسته‌بندی')
    }
  }

  const handleCancel = () => {
    setFormData({ name: '', slug: '' })
    setEditingId(null)
    setShowForm(false)
  }

  const generateSlug = (name: string) => {
    return name
      .toLowerCase()
      .replace(/[\s\u0600-\u06FF]+/g, '-')
      .replace(/[^a-z0-9\u0600-\u06FF-]/g, '')
      .replace(/-+/g, '-')
      .replace(/^-|-$/g, '')
  }

  const handleNameChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const name = e.target.value
    setFormData(prev => ({
      name,
      slug: prev.slug || generateSlug(name)
    }))
  }

  if (loading) {
    return <div className="loading">در حال بارگذاری...</div>
  }

  return (
    <div className="admin-page">
      <div className="admin-header">
        <h1>مدیریت دسته‌بندی مقالات</h1>
        {!showForm && (
          <button 
            onClick={() => setShowForm(true)} 
            className="btn btn-primary"
          >
            دسته‌بندی جدید
          </button>
        )}
      </div>

      {error && <div className="error-message">{error}</div>}

      {showForm && (
        <form onSubmit={handleSubmit} className="admin-form compact-form">
          <h3>{editingId ? 'ویرایش دسته‌بندی' : 'دسته‌بندی جدید'}</h3>
          
          <div className="form-group">
            <label htmlFor="name">نام *</label>
            <input
              type="text"
              id="name"
              value={formData.name}
              onChange={handleNameChange}
              required
              className="form-input"
              placeholder="نام دسته‌بندی"
            />
          </div>

          <div className="form-group">
            <label htmlFor="slug">اسلاگ *</label>
            <input
              type="text"
              id="slug"
              value={formData.slug}
              onChange={(e) => setFormData(prev => ({ ...prev, slug: e.target.value }))}
              required
              className="form-input"
              placeholder="category-slug"
            />
          </div>

          <div className="form-actions">
            <button type="submit" disabled={saving} className="btn btn-primary">
              {saving ? 'در حال ذخیره...' : 'ذخیره'}
            </button>
            <button type="button" onClick={handleCancel} className="btn btn-secondary">
              انصراف
            </button>
          </div>
        </form>
      )}

      {categories.length === 0 && !showForm ? (
        <div className="empty-state">
          <p>دسته‌بندی‌ وجود ندارد.</p>
          <button 
            onClick={() => setShowForm(true)} 
            className="btn btn-primary"
          >
            اولین دسته‌بندی را بسازید
          </button>
        </div>
      ) : (
        <table className="admin-table">
          <thead>
            <tr>
              <th>نام</th>
              <th>اسلاگ</th>
              <th>تاریخ ایجاد</th>
              <th>عملیات</th>
            </tr>
          </thead>
          <tbody>
            {categories.map((cat) => (
              <tr key={cat.id}>
                <td>{cat.name}</td>
                <td>{cat.slug}</td>
                <td>{new Date(cat.created_at).toLocaleDateString('fa-IR')}</td>
                <td>
                  <div className="action-buttons">
                    <button 
                      onClick={() => handleEdit(cat)} 
                      className="btn btn-sm"
                    >
                      ویرایش
                    </button>
                    <button 
                      onClick={() => handleDelete(cat.id)} 
                      className="btn btn-sm btn-danger"
                    >
                      حذف
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}