import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getProfile, updateProfile } from '../api/account'
import type { Profile, ProfileUpdateInput } from '../api/account'
import { ApiError } from '../api/errors'
import { useAuth } from '../hooks/useAuth'

export default function ProfilePage() {
  const { user } = useAuth()
  const [profile, setProfile] = useState<Profile | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  const [form, setForm] = useState<ProfileUpdateInput>({
    name: '',
    phone: '',
  })

  useEffect(() => {
    if (!user) return

    let ignore = false

    getProfile()
      .then((data) => {
        if (!ignore) {
          setProfile(data)
          setForm({
            name: data.name,
            phone: data.phone || '',
          })
        }
      })
      .catch((err) => {
        if (!ignore) {
          setError(err instanceof Error ? err.message : 'مشکلی در بارگذاری اطلاعات شخصی پیش آمد')
        }
      })
      .finally(() => {
        if (!ignore) setLoading(false)
      })

    return () => {
      ignore = true
    }
  }, [user])

  const handleChange = (field: keyof ProfileUpdateInput) => (event: React.ChangeEvent<HTMLInputElement>) => {
    setForm((prev) => ({ ...prev, [field]: event.target.value }))
  }

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    setSaving(true)
    setError('')
    setSuccess('')

    try {
      const updated = await updateProfile(form)
      setProfile(updated)
      setSuccess('اطلاعات با موفقیت به‌روزرسانی شد.')
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message || 'خطا در به‌روزرسانی اطلاعات')
      } else {
        setError('خطای نامشخصی رخ داد.')
      }
    } finally {
      setSaving(false)
    }
  }

  if (!user) {
    return (
      <main>
        <h1>اطلاعات شخصی</h1>
        <p className="empty-state">
          برای ویرایش اطلاعات شخصی، <Link to="/login">وارد شوید</Link>.
        </p>
      </main>
    )
  }

  if (loading) {
    return (
      <main>
        <h1>اطلاعات شخصی</h1>
        <p className="state-message">در حال بارگذاری...</p>
      </main>
    )
  }

  return (
    <main>
      <h1>اطلاعات شخصی</h1>
      
      <div className="account-layout">
        <aside className="account-sidebar">
          <nav className="account-nav">
            <h3 className="account-nav-title">مدیریت حساب</h3>
            <ul>
              <li className="account-nav-item">
                <Link to="/account">نمای کلی</Link>
              </li>
              <li className="account-nav-item active">
                <Link to="/account/profile">اطلاعات شخصی</Link>
              </li>
              <li className="account-nav-item">
                <Link to="/account/addresses">آدرس‌ها</Link>
              </li>
              <li className="account-nav-item">
                <Link to="/orders">سفارش‌ها</Link>
              </li>
            </ul>
          </nav>
        </aside>

        <div className="account-content">
          <div className="form-card">
            <form onSubmit={handleSubmit}>
              {error && (
                <p className="alert alert-error" role="alert">
                  {error}
                </p>
              )}
              
              {success && (
                <p className="alert alert-success" role="alert">
                  {success}
                </p>
              )}

              <div className="form-field">
                <label htmlFor="name">نام</label>
                <input
                  id="name"
                  type="text"
                  required
                  maxLength={255}
                  value={form.name}
                  onChange={handleChange('name')}
                  disabled={saving}
                />
              </div>

              <div className="form-field">
                <label htmlFor="email">ایمیل</label>
                <input
                  id="email"
                  type="email"
                  value={profile?.email || ''}
                  disabled
                  className="disabled-input"
                />
                <p className="form-hint">ایمیل قابل تغییر نیست.</p>
              </div>

              <div className="form-field">
                <label htmlFor="phone">شماره موبایل</label>
                <input
                  id="phone"
                  type="tel"
                  maxLength={20}
                  placeholder="09xxxxxxxxx"
                  inputMode="tel"
                  value={form.phone}
                  onChange={handleChange('phone')}
                  disabled={saving}
                />
                <p className="form-hint">شماره موبایل اختیاری است.</p>
              </div>

              <div className="form-actions">
                <button type="submit" className="btn btn-primary" disabled={saving}>
                  {saving ? 'در حال ذخیره...' : 'ذخیره تغییرات'}
                </button>
                <Link to="/account" className="btn btn-secondary">
                  انصراف
                </Link>
              </div>
            </form>
          </div>

          <div className="account-info-card">
            <h3>اطلاعات حساب</h3>
            <div className="account-info-content">
              <p>
                <strong>نقش:</strong> {profile?.role === 'admin' ? 'مدیر' : 'کاربر'}
              </p>
              <p>
                <strong>تاریخ عضویت:</strong> {profile?.created_at ? new Date(profile.created_at).toLocaleDateString('fa-IR') : ''}
              </p>
              <p>
                <strong>آخرین به‌روزرسانی:</strong> {profile?.updated_at ? new Date(profile.updated_at).toLocaleDateString('fa-IR') : ''}
              </p>
            </div>
          </div>
        </div>
      </div>
    </main>
  )
}