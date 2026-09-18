import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { Link } from 'react-router-dom'
import {
  createCategory,
  deleteCategory,
  getAdminCategories,
  updateCategory,
} from '../../api/categories'
import { ApiError } from '../../api/errors'
import type { Category } from '../../types/category'

export default function AdminCategoriesPage() {
  const [categories, setCategories] = useState<Category[] | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [reloadKey, setReloadKey] = useState(0)

  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [formError, setFormError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const [editingId, setEditingId] = useState<number | null>(null)
  const [editName, setEditName] = useState('')
  const [editSlug, setEditSlug] = useState('')
  const [editError, setEditError] = useState('')

  const [deletingId, setDeletingId] = useState<number | null>(null)
  const [deleteError, setDeleteError] = useState('')

  const reload = () => {
    setLoading(true)
    setError('')
    setReloadKey((key) => key + 1)
  }

  useEffect(() => {
    let ignore = false

    getAdminCategories()
      .then((data) => {
        if (!ignore) setCategories(data)
      })
      .catch((err) => {
        if (!ignore) setError(err instanceof Error ? err.message : 'مشکلی در بارگذاری دسته‌بندی‌ها پیش آمد')
      })
      .finally(() => {
        if (!ignore) setLoading(false)
      })

    return () => {
      ignore = true
    }
  }, [reloadKey])

  const handleCreate = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setFormError('')

    const trimmedName = name.trim()
    const trimmedSlug = slug.trim()
    if (!trimmedName || !trimmedSlug) {
      setFormError('نام و Slug الزامی است')
      return
    }

    setSubmitting(true)
    try {
      await createCategory({ name: trimmedName, slug: trimmedSlug })
      setName('')
      setSlug('')
      reload()
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setFormError('این Slug قبلاً استفاده شده است')
      } else {
        setFormError(err instanceof Error ? err.message : 'مشکلی در ایجاد دسته‌بندی پیش آمد')
      }
    } finally {
      setSubmitting(false)
    }
  }

  const startEdit = (category: Category) => {
    setEditingId(category.id)
    setEditName(category.name)
    setEditSlug(category.slug)
    setEditError('')
  }

  const cancelEdit = () => {
    setEditingId(null)
    setEditError('')
  }

  const handleUpdate = async (id: number) => {
    setEditError('')
    const trimmedName = editName.trim()
    const trimmedSlug = editSlug.trim()
    if (!trimmedName || !trimmedSlug) {
      setEditError('نام و Slug الزامی است')
      return
    }

    try {
      await updateCategory(id, { name: trimmedName, slug: trimmedSlug })
      setEditingId(null)
      reload()
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setEditError('این Slug قبلاً استفاده شده است')
      } else {
        setEditError(err instanceof Error ? err.message : 'مشکلی در ذخیره دسته‌بندی پیش آمد')
      }
    }
  }

  const handleDelete = async (category: Category) => {
    const confirmed = window.confirm(`دسته‌بندی «${category.name}» حذف شود؟`)
    if (!confirmed) return

    setDeleteError('')
    setDeletingId(category.id)

    try {
      await deleteCategory(category.id)
      setCategories((current) => (current ? current.filter((c) => c.id !== category.id) : current))
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setDeleteError(`دسته‌بندی «${category.name}» به دلیل استفاده در محصولات قابل حذف نیست.`)
      } else {
        setDeleteError(err instanceof Error ? err.message : 'مشکلی در حذف دسته‌بندی پیش آمد')
      }
    } finally {
      setDeletingId(null)
    }
  }

  return (
    <main>
      <div className="page-header">
        <h1>مدیریت · دسته‌بندی‌ها</h1>
        <p className="page-subtitle">مدیریت دسته‌بندی‌های محصولات فروشگاه.</p>
      </div>

      <Link to="/admin/products" className="back-link">
        → بازگشت به محصولات
      </Link>

      <div className="form-card" style={{ maxWidth: 480 }}>
        <h2>دسته‌بندی جدید</h2>
        <form onSubmit={handleCreate}>
          <div className="form-field">
            <label htmlFor="category-name">نام</label>
            <input id="category-name" type="text" value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div className="form-field">
            <label htmlFor="category-slug">Slug</label>
            <input id="category-slug" type="text" value={slug} onChange={(e) => setSlug(e.target.value)} required />
          </div>
          {formError && (
            <p className="alert alert-error" role="alert">
              {formError}
            </p>
          )}
          <button type="submit" className="btn btn-primary btn-block" disabled={submitting}>
            {submitting ? 'در حال ایجاد...' : 'ایجاد دسته‌بندی'}
          </button>
        </form>
      </div>

      {deleteError && (
        <p className="alert alert-error" role="alert">
          {deleteError}
        </p>
      )}

      {loading && <p className="state-message">در حال بارگذاری دسته‌بندی‌ها...</p>}

      {!loading && error && (
        <>
          <p className="alert alert-error" role="alert">
            {error}
          </p>
          <button type="button" className="btn btn-secondary" onClick={reload}>
            تلاش دوباره
          </button>
        </>
      )}

      {!loading && !error && categories && categories.length === 0 && (
        <p className="empty-state">هنوز دسته‌بندی‌ای ثبت نشده است.</p>
      )}

      {!loading && !error && categories && categories.length > 0 && (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>نام</th>
                <th>Slug</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {categories.map((category) => (
                <tr key={category.id}>
                  {editingId === category.id ? (
                    <>
                      <td>
                        <input
                          type="text"
                          value={editName}
                          onChange={(e) => setEditName(e.target.value)}
                          style={{ width: '100%' }}
                        />
                      </td>
                      <td>
                        <input
                          type="text"
                          value={editSlug}
                          onChange={(e) => setEditSlug(e.target.value)}
                          style={{ width: '100%' }}
                        />
                      </td>
                      <td>
                        <div style={{ display: 'flex', gap: 8, flexDirection: 'column' }}>
                          {editError && (
                            <p className="alert alert-error" role="alert" style={{ margin: 0 }}>
                              {editError}
                            </p>
                          )}
                          <div style={{ display: 'flex', gap: 8 }}>
                            <button type="button" className="btn btn-primary btn-sm" onClick={() => handleUpdate(category.id)}>
                              ذخیره
                            </button>
                            <button type="button" className="btn btn-secondary btn-sm" onClick={cancelEdit}>
                              انصراف
                            </button>
                          </div>
                        </div>
                      </td>
                    </>
                  ) : (
                    <>
                      <td>{category.name}</td>
                      <td>{category.slug}</td>
                      <td>
                        <div style={{ display: 'flex', gap: 8 }}>
                          <button type="button" className="btn btn-secondary btn-sm" onClick={() => startEdit(category)}>
                            ویرایش
                          </button>
                          <button
                            type="button"
                            className="btn btn-danger btn-sm"
                            onClick={() => handleDelete(category)}
                            disabled={deletingId === category.id}
                          >
                            {deletingId === category.id ? 'در حال حذف...' : 'حذف'}
                          </button>
                        </div>
                      </td>
                    </>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </main>
  )
}
