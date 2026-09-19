import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { listAddresses, createAddress, updateAddress, deleteAddress } from '../api/addresses'
import type { Address, AddressCreateInput, AddressUpdateInput } from '../api/addresses'
import { useAuth } from '../hooks/useAuth'

export default function AddressesPage() {
  const { user } = useAuth()
  const [addresses, setAddresses] = useState<Address[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [formError, setFormError] = useState('')
  const [formSuccess, setFormSuccess] = useState('')

  const emptyForm: AddressCreateInput = {
    label: '',
    recipient_name: '',
    phone: '',
    address_line1: '',
    address_line2: '',
    city: '',
    postal_code: '',
    country: 'ایران',
    is_default: false,
  }

  const [form, setForm] = useState<AddressCreateInput>(emptyForm)

  const loadAddresses = useCallback(() => {
    listAddresses()
      .then((data) => {
        setAddresses(data)
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : 'مشکلی در بارگذاری آدرس‌ها پیش آمد')
      })
      .finally(() => {
        setLoading(false)
      })
  }, [])

  useEffect(() => {
    if (!user) return

    loadAddresses()
  }, [user, loadAddresses])

  const handleChange = (field: keyof AddressCreateInput) => (event: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    setForm((prev) => ({ ...prev, [field]: event.target.value }))
  }

  const handleCheckboxChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setForm((prev) => ({ ...prev, is_default: event.target.checked }))
  }

  const handleEdit = (address: Address) => {
    setEditingId(address.id)
    setForm({
      label: address.label,
      recipient_name: address.recipient_name,
      phone: address.phone,
      address_line1: address.address_line1,
      address_line2: address.address_line2 || '',
      city: address.city,
      postal_code: address.postal_code,
      country: address.country,
      is_default: address.is_default,
    })
    setShowForm(true)
    setFormError('')
    setFormSuccess('')
  }

  const handleDelete = async (id: number) => {
    if (!confirm('آیا مطمئن هستید که می‌خواهید این آدرس را حذف کنید؟')) {
      return
    }

    try {
      await deleteAddress(id)
      loadAddresses()
      setFormSuccess('آدرس با موفقیت حذف شد.')
      setTimeout(() => setFormSuccess(''), 3000)
    } catch (err) {
      if (err instanceof Error) {
        setFormError(err.message)
      } else {
        setFormError('خطا در حذف آدرس')
      }
    }
  }

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    setFormError('')
    setFormSuccess('')

    try {
      if (editingId) {
        await updateAddress(editingId, form as AddressUpdateInput)
        setFormSuccess('آدرس با موفقیت به‌روزرسانی شد.')
      } else {
        await createAddress(form)
        setFormSuccess('آدرس جدید با موفقیت اضافه شد.')
      }

      // Reset form and reload addresses
      setForm(emptyForm)
      setShowForm(false)
      setEditingId(null)
      loadAddresses()

      setTimeout(() => setFormSuccess(''), 3000)
    } catch (err) {
      if (err instanceof Error) {
        setFormError(err.message || 'خطا در ذخیره آدرس')
      } else {
        setFormError('خطای نامشخصی رخ داد.')
      }
    }
  }

  const handleCancel = () => {
    setForm(emptyForm)
    setShowForm(false)
    setEditingId(null)
    setFormError('')
    setFormSuccess('')
  }

  if (!user) {
    return (
      <main>
        <h1>آدرس‌ها</h1>
        <p className="empty-state">
          برای مدیریت آدرس‌ها، <Link to="/login">وارد شوید</Link>.
        </p>
      </main>
    )
  }

  if (loading) {
    return (
      <main>
        <h1>آدرس‌ها</h1>
        <p className="state-message">در حال بارگذاری...</p>
      </main>
    )
  }

  return (
    <main>
      <h1>آدرس‌ها</h1>
      
      <div className="account-layout">
        <aside className="account-sidebar">
          <nav className="account-nav">
            <h3 className="account-nav-title">مدیریت حساب</h3>
            <ul>
              <li className="account-nav-item">
                <Link to="/account">نمای کلی</Link>
              </li>
              <li className="account-nav-item">
                <Link to="/account/profile">اطلاعات شخصی</Link>
              </li>
              <li className="account-nav-item active">
                <Link to="/account/addresses">آدرس‌ها</Link>
              </li>
              <li className="account-nav-item">
                <Link to="/orders">سفارش‌ها</Link>
              </li>
            </ul>
          </nav>
        </aside>

        <div className="account-content">
          {error && (
            <p className="alert alert-error" role="alert">
              {error}
            </p>
          )}

          {formError && (
            <p className="alert alert-error" role="alert">
              {formError}
            </p>
          )}

          {formSuccess && (
            <p className="alert alert-success" role="alert">
              {formSuccess}
            </p>
          )}

          <div className="addresses-header">
            <h2>آدرس‌های ذخیره شده</h2>
            <button
              type="button"
              className="btn btn-primary"
              onClick={() => {
                setShowForm(true)
                setEditingId(null)
                setForm(emptyForm)
              }}
              disabled={showForm}
            >
              + افزودن آدرس جدید
            </button>
          </div>

          {showForm && (
            <div className="form-card">
              <h3>{editingId ? 'ویرایش آدرس' : 'افزودن آدرس جدید'}</h3>
              <form onSubmit={handleSubmit}>
                <div className="form-field">
                  <label htmlFor="label">عنوان آدرس (اختیاری)</label>
                  <input
                    id="label"
                    type="text"
                    maxLength={255}
                    placeholder="مثال: خانه، محل کار"
                    value={form.label}
                    onChange={handleChange('label')}
                  />
                </div>

                <div className="form-field">
                  <label htmlFor="recipient_name">نام گیرنده *</label>
                  <input
                    id="recipient_name"
                    type="text"
                    required
                    maxLength={255}
                    value={form.recipient_name}
                    onChange={handleChange('recipient_name')}
                  />
                </div>

                <div className="form-field">
                  <label htmlFor="phone">شماره موبایل *</label>
                  <input
                    id="phone"
                    type="tel"
                    required
                    maxLength={20}
                    placeholder="09xxxxxxxxx"
                    inputMode="tel"
                    value={form.phone}
                    onChange={handleChange('phone')}
                  />
                </div>

                <div className="form-field">
                  <label htmlFor="address_line1">آدرس (خط اول) *</label>
                  <input
                    id="address_line1"
                    type="text"
                    required
                    maxLength={255}
                    value={form.address_line1}
                    onChange={handleChange('address_line1')}
                  />
                </div>

                <div className="form-field">
                  <label htmlFor="address_line2">ادامه آدرس (اختیاری)</label>
                  <input
                    id="address_line2"
                    type="text"
                    maxLength={255}
                    value={form.address_line2}
                    onChange={handleChange('address_line2')}
                  />
                </div>

                <div className="form-field">
                  <label htmlFor="city">شهر *</label>
                  <input
                    id="city"
                    type="text"
                    required
                    maxLength={128}
                    value={form.city}
                    onChange={handleChange('city')}
                  />
                </div>

                <div className="form-field">
                  <label htmlFor="postal_code">کد پستی *</label>
                  <input
                    id="postal_code"
                    type="text"
                    required
                    maxLength={20}
                    inputMode="numeric"
                    value={form.postal_code}
                    onChange={handleChange('postal_code')}
                  />
                </div>

                <div className="form-field">
                  <label htmlFor="country">کشور *</label>
                  <input
                    id="country"
                    type="text"
                    required
                    maxLength={128}
                    value={form.country}
                    onChange={handleChange('country')}
                  />
                </div>

                <div className="form-field checkbox-field">
                  <label>
                    <input
                      type="checkbox"
                      checked={form.is_default}
                      onChange={handleCheckboxChange}
                    />
                    تنظیم به عنوان آدرس پیش‌فرض
                  </label>
                  <p className="form-hint">
                    آدرس پیش‌فرض به طور خودکار در صفحه تسویه حساب انتخاب می‌شود.
                  </p>
                </div>

                <div className="form-actions">
                  <button type="submit" className="btn btn-primary">
                    {editingId ? 'ذخیره تغییرات' : 'افزودن آدرس'}
                  </button>
                  <button type="button" className="btn btn-secondary" onClick={handleCancel}>
                    انصراف
                  </button>
                </div>
              </form>
            </div>
          )}

          {addresses.length === 0 ? (
            <div className="empty-state-card">
              <p>هنوز آدرسی ثبت نکرده‌اید.</p>
              {!showForm && (
                <button
                  type="button"
                  className="btn btn-primary"
                  onClick={() => setShowForm(true)}
                >
                  افزودن اولین آدرس
                </button>
              )}
            </div>
          ) : (
            <div className="addresses-list">
              {addresses.map((address) => (
                <div key={address.id} className="address-card">
                  <div className="address-card-header">
                    <h4>
                      {address.label || 'آدرس بدون عنوان'}
                      {address.is_default && <span className="default-badge">پیش‌فرض</span>}
                    </h4>
                    <div className="address-card-actions">
                      <button
                        type="button"
                        className="btn btn-secondary btn-sm"
                        onClick={() => handleEdit(address)}
                      >
                        ویرایش
                      </button>
                      <button
                        type="button"
                        className="btn btn-danger btn-sm"
                        onClick={() => handleDelete(address.id)}
                        disabled={address.is_default}
                      >
                        حذف
                      </button>
                    </div>
                  </div>
                  <div className="address-card-content">
                    <p>
                      <strong>گیرنده:</strong> {address.recipient_name}
                    </p>
                    <p>
                      <strong>موبایل:</strong> {address.phone}
                    </p>
                    <p>
                      <strong>آدرس:</strong> {address.address_line1}
                      {address.address_line2 && `، ${address.address_line2}`}
                    </p>
                    <p>
                      <strong>شهر:</strong> {address.city}، کد پستی: {address.postal_code}
                    </p>
                    <p>
                      <strong>کشور:</strong> {address.country}
                    </p>
                    <p className="address-date">
                      <small>
                        {new Date(address.created_at).toLocaleDateString('fa-IR')}
                      </small>
                    </p>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </main>
  )
}