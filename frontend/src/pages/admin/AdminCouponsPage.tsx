import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { createCoupon, getAdminCoupons, updateCoupon } from '../../api/coupons'
import { ApiError } from '../../api/errors'
import type { AdminCoupon, DiscountType } from '../../types/coupon'
import { formatDateFa, formatToman } from '../../lib/format'

// Local datetime-input <-> ISO helpers. <input type="datetime-local"> works
// with "YYYY-MM-DDTHH:mm" (no timezone); we treat it as the admin's local
// time and convert to/from a full ISO string for the API.
function toDatetimeLocalValue(iso: string | null): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function fromDatetimeLocalValue(value: string): string | null {
  if (!value) return null
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return null
  return d.toISOString()
}

type FormState = {
  code: string
  discount_type: DiscountType
  value: string
  min_order_amount: string
  usage_limit: string
  per_user_limit: string
  starts_at: string
  ends_at: string
}

function emptyForm(): FormState {
  return {
    code: '',
    discount_type: 'percent',
    value: '',
    min_order_amount: '0',
    usage_limit: '',
    per_user_limit: '',
    starts_at: '',
    ends_at: '',
  }
}

// couponToForm converts a Coupon (canonical IRR) into form state, showing
// fixed monetary fields in Toman (IRR / 10) to match the input labels and
// every other price field in the admin UI. Percent values are unitless
// and pass through unchanged.
function couponToForm(c: AdminCoupon): FormState {
  return {
    code: c.code,
    discount_type: c.discount_type,
    value: String(c.discount_type === 'fixed' ? c.value / 10 : c.value),
    min_order_amount: String(c.min_order_amount / 10),
    usage_limit: c.usage_limit != null ? String(c.usage_limit) : '',
    per_user_limit: c.per_user_limit != null ? String(c.per_user_limit) : '',
    starts_at: toDatetimeLocalValue(c.starts_at),
    ends_at: toDatetimeLocalValue(c.ends_at),
  }
}

export default function AdminCouponsPage() {
  const [coupons, setCoupons] = useState<AdminCoupon[] | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [reloadKey, setReloadKey] = useState(0)

  const [form, setForm] = useState<FormState>(emptyForm())
  const [formError, setFormError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const [editingId, setEditingId] = useState<number | null>(null)
  const [editForm, setEditForm] = useState<FormState>(emptyForm())
  const [editError, setEditError] = useState('')
  const [savingEdit, setSavingEdit] = useState(false)

  const [togglingId, setTogglingId] = useState<number | null>(null)
  const [toggleError, setToggleError] = useState('')

  const reload = () => {
    setLoading(true)
    setError('')
    setReloadKey((key) => key + 1)
  }

  useEffect(() => {
    let ignore = false

    getAdminCoupons()
      .then((data) => {
        if (!ignore) setCoupons(data)
      })
      .catch((err) => {
        if (!ignore) setError(err instanceof Error ? err.message : 'مشکلی در بارگذاری کدهای تخفیف پیش آمد')
      })
      .finally(() => {
        if (!ignore) setLoading(false)
      })

    return () => {
      ignore = true
    }
  }, [reloadKey])

  // Fixed monetary fields (value when discount_type is "fixed", and
  // min_order_amount) are entered by the admin in Toman (matching every
  // other price input's display convention) and converted to the
  // canonical IRR integer here before sending to the API. Percent values
  // are unitless and never converted.
  const parseFormToInput = (f: FormState) => {
    const rawValue = Number(f.value)
    const value = f.discount_type === 'fixed' ? rawValue * 10 : rawValue
    const minOrderAmount = Number(f.min_order_amount || '0') * 10
    const usageLimit = f.usage_limit.trim() === '' ? null : Number(f.usage_limit)
    const perUserLimit = f.per_user_limit.trim() === '' ? null : Number(f.per_user_limit)
    return {
      code: f.code.trim(),
      discount_type: f.discount_type,
      value,
      min_order_amount: minOrderAmount,
      usage_limit: usageLimit,
      per_user_limit: perUserLimit,
      starts_at: fromDatetimeLocalValue(f.starts_at),
      ends_at: fromDatetimeLocalValue(f.ends_at),
      is_active: true,
    }
  }

  const validateForm = (f: FormState): string => {
    if (!f.code.trim()) return 'کد تخفیف الزامی است'
    if (!f.value.trim() || Number.isNaN(Number(f.value))) return 'مقدار تخفیف نامعتبر است'
    if (f.discount_type === 'percent' && (Number(f.value) < 1 || Number(f.value) > 100)) {
      return 'درصد تخفیف باید بین ۱ تا ۱۰۰ باشد'
    }
    if (f.discount_type === 'fixed' && Number(f.value) <= 0) {
      return 'مقدار تخفیف ثابت باید بیشتر از صفر باشد'
    }
    if (f.min_order_amount.trim() !== '' && Number(f.min_order_amount) < 0) {
      return 'حداقل مبلغ سفارش نمی‌تواند منفی باشد'
    }
    if (f.usage_limit.trim() !== '' && Number(f.usage_limit) <= 0) {
      return 'سقف استفاده کل باید بیشتر از صفر باشد'
    }
    if (f.per_user_limit.trim() !== '' && Number(f.per_user_limit) <= 0) {
      return 'سقف استفاده هر کاربر باید بیشتر از صفر باشد'
    }
    if (f.starts_at && f.ends_at && new Date(f.starts_at) >= new Date(f.ends_at)) {
      return 'تاریخ شروع باید قبل از تاریخ پایان باشد'
    }
    return ''
  }

  const handleCreate = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const validationError = validateForm(form)
    if (validationError) {
      setFormError(validationError)
      return
    }
    setFormError('')
    setSubmitting(true)
    try {
      await createCoupon(parseFormToInput(form))
      setForm(emptyForm())
      reload()
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setFormError('این کد تخفیف قبلاً استفاده شده است')
      } else {
        setFormError(err instanceof Error ? err.message : 'مشکلی در ایجاد کد تخفیف پیش آمد')
      }
    } finally {
      setSubmitting(false)
    }
  }

  const startEdit = (c: AdminCoupon) => {
    setEditingId(c.id)
    setEditForm(couponToForm(c))
    setEditError('')
  }

  const cancelEdit = () => {
    setEditingId(null)
    setEditError('')
  }

  const handleSaveEdit = async (id: number) => {
    const validationError = validateForm(editForm)
    if (validationError) {
      setEditError(validationError)
      return
    }
    setEditError('')
    setSavingEdit(true)
    try {
      await updateCoupon(id, parseFormToInput(editForm))
      setEditingId(null)
      reload()
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setEditError('این کد تخفیف قبلاً استفاده شده است')
      } else {
        setEditError(err instanceof Error ? err.message : 'مشکلی در ذخیره کد تخفیف پیش آمد')
      }
    } finally {
      setSavingEdit(false)
    }
  }

  const handleToggleActive = async (c: AdminCoupon) => {
    setToggleError('')
    setTogglingId(c.id)
    try {
      await updateCoupon(c.id, { is_active: !c.is_active })
      reload()
    } catch (err) {
      setToggleError(err instanceof Error ? err.message : 'مشکلی در تغییر وضعیت کد تخفیف پیش آمد')
    } finally {
      setTogglingId(null)
    }
  }

  return (
    <main>
      <div className="page-header">
        <h1>مدیریت · کدهای تخفیف</h1>
        <p className="page-subtitle">ایجاد و مدیریت کدهای تخفیف فروشگاه.</p>
      </div>

      <div className="form-card" style={{ maxWidth: 560 }}>
        <h2>کد تخفیف جدید</h2>
        <form onSubmit={handleCreate}>
          <div className="form-field">
            <label htmlFor="coupon-code">کد</label>
            <input
              id="coupon-code"
              type="text"
              value={form.code}
              onChange={(e) => setForm((prev) => ({ ...prev, code: e.target.value }))}
              required
            />
          </div>

          <div className="form-field">
            <label htmlFor="coupon-type">نوع تخفیف</label>
            <select
              id="coupon-type"
              value={form.discount_type}
              onChange={(e) => setForm((prev) => ({ ...prev, discount_type: e.target.value as DiscountType }))}
            >
              <option value="percent">درصدی</option>
              <option value="fixed">مقدار ثابت (تومان)</option>
            </select>
          </div>

          <div className="form-field">
            <label htmlFor="coupon-value">
              {form.discount_type === 'percent' ? 'درصد تخفیف (۱ تا ۱۰۰)' : 'مقدار تخفیف (تومان)'}
            </label>
            <input
              id="coupon-value"
              type="number"
              min={form.discount_type === 'percent' ? 1 : 1}
              max={form.discount_type === 'percent' ? 100 : undefined}
              value={form.value}
              onChange={(e) => setForm((prev) => ({ ...prev, value: e.target.value }))}
              required
            />
          </div>

          <div className="form-field">
            <label htmlFor="coupon-min-order">حداقل مبلغ سفارش (تومان)</label>
            <input
              id="coupon-min-order"
              type="number"
              min={0}
              value={form.min_order_amount}
              onChange={(e) => setForm((prev) => ({ ...prev, min_order_amount: e.target.value }))}
            />
          </div>

          <div className="form-field">
            <label htmlFor="coupon-usage-limit">سقف استفاده کل (اختیاری)</label>
            <input
              id="coupon-usage-limit"
              type="number"
              min={1}
              value={form.usage_limit}
              onChange={(e) => setForm((prev) => ({ ...prev, usage_limit: e.target.value }))}
              placeholder="بدون محدودیت"
            />
          </div>

          <div className="form-field">
            <label htmlFor="coupon-per-user-limit">سقف استفاده هر کاربر (اختیاری)</label>
            <input
              id="coupon-per-user-limit"
              type="number"
              min={1}
              value={form.per_user_limit}
              onChange={(e) => setForm((prev) => ({ ...prev, per_user_limit: e.target.value }))}
              placeholder="بدون محدودیت"
            />
          </div>

          <div className="form-field">
            <label htmlFor="coupon-starts-at">تاریخ شروع (اختیاری)</label>
            <input
              id="coupon-starts-at"
              type="datetime-local"
              value={form.starts_at}
              onChange={(e) => setForm((prev) => ({ ...prev, starts_at: e.target.value }))}
            />
          </div>

          <div className="form-field">
            <label htmlFor="coupon-ends-at">تاریخ پایان (اختیاری)</label>
            <input
              id="coupon-ends-at"
              type="datetime-local"
              value={form.ends_at}
              onChange={(e) => setForm((prev) => ({ ...prev, ends_at: e.target.value }))}
            />
          </div>

          {formError && (
            <p className="alert alert-error" role="alert">
              {formError}
            </p>
          )}

          <button type="submit" className="btn btn-primary btn-block" disabled={submitting}>
            {submitting ? 'در حال ایجاد...' : 'ایجاد کد تخفیف'}
          </button>
        </form>
      </div>

      {toggleError && (
        <p className="alert alert-error" role="alert">
          {toggleError}
        </p>
      )}

      {loading && <p className="state-message">در حال بارگذاری کدهای تخفیف...</p>}

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

      {!loading && !error && coupons && coupons.length === 0 && (
        <p className="empty-state">هنوز کد تخفیفی ثبت نشده است.</p>
      )}

      {!loading && !error && coupons && coupons.length > 0 && (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>کد</th>
                <th>نوع</th>
                <th>مقدار</th>
                <th>حداقل خرید</th>
                <th>سقف کل</th>
                <th>سقف هر کاربر</th>
                <th>شروع</th>
                <th>پایان</th>
                <th>استفاده شده</th>
                <th>وضعیت</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {coupons.map((c) => (
                <tr key={c.id}>
                  {editingId === c.id ? (
                    <>
                      <td>
                        <input
                          type="text"
                          value={editForm.code}
                          onChange={(e) => setEditForm((prev) => ({ ...prev, code: e.target.value }))}
                          style={{ width: '100%' }}
                        />
                      </td>
                      <td>
                        <select
                          value={editForm.discount_type}
                          onChange={(e) =>
                            setEditForm((prev) => ({ ...prev, discount_type: e.target.value as DiscountType }))
                          }
                        >
                          <option value="percent">درصدی</option>
                          <option value="fixed">مقدار ثابت</option>
                        </select>
                      </td>
                      <td>
                        <input
                          type="number"
                          value={editForm.value}
                          onChange={(e) => setEditForm((prev) => ({ ...prev, value: e.target.value }))}
                          style={{ width: 90 }}
                        />
                      </td>
                      <td>
                        <input
                          type="number"
                          value={editForm.min_order_amount}
                          onChange={(e) => setEditForm((prev) => ({ ...prev, min_order_amount: e.target.value }))}
                          style={{ width: 100 }}
                        />
                      </td>
                      <td>
                        <input
                          type="number"
                          value={editForm.usage_limit}
                          onChange={(e) => setEditForm((prev) => ({ ...prev, usage_limit: e.target.value }))}
                          placeholder="بدون محدودیت"
                          style={{ width: 90 }}
                        />
                      </td>
                      <td>
                        <input
                          type="number"
                          value={editForm.per_user_limit}
                          onChange={(e) => setEditForm((prev) => ({ ...prev, per_user_limit: e.target.value }))}
                          placeholder="بدون محدودیت"
                          style={{ width: 90 }}
                        />
                      </td>
                      <td>
                        <input
                          type="datetime-local"
                          value={editForm.starts_at}
                          onChange={(e) => setEditForm((prev) => ({ ...prev, starts_at: e.target.value }))}
                        />
                      </td>
                      <td>
                        <input
                          type="datetime-local"
                          value={editForm.ends_at}
                          onChange={(e) => setEditForm((prev) => ({ ...prev, ends_at: e.target.value }))}
                        />
                      </td>
                      <td>{c.usage_count}</td>
                      <td>{c.is_active ? 'فعال' : 'غیرفعال'}</td>
                      <td>
                        <div style={{ display: 'flex', gap: 8, flexDirection: 'column' }}>
                          {editError && (
                            <p className="alert alert-error" role="alert" style={{ margin: 0 }}>
                              {editError}
                            </p>
                          )}
                          <div style={{ display: 'flex', gap: 8 }}>
                            <button
                              type="button"
                              className="btn btn-primary btn-sm"
                              onClick={() => handleSaveEdit(c.id)}
                              disabled={savingEdit}
                            >
                              {savingEdit ? 'در حال ذخیره...' : 'ذخیره'}
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
                      <td>{c.code}</td>
                      <td>{c.discount_type === 'percent' ? 'درصدی' : 'مقدار ثابت'}</td>
                      <td>{c.discount_type === 'percent' ? `${c.value}%` : formatToman(c.value)}</td>
                      <td>{formatToman(c.min_order_amount)}</td>
                      <td>{c.usage_limit ?? 'بدون محدودیت'}</td>
                      <td>{c.per_user_limit ?? 'بدون محدودیت'}</td>
                      <td>{c.starts_at ? formatDateFa(c.starts_at) : '—'}</td>
                      <td>{c.ends_at ? formatDateFa(c.ends_at) : '—'}</td>
                      <td>{c.usage_count}</td>
                      <td>
                        <span className="status-badge">{c.is_active ? 'فعال' : 'غیرفعال'}</span>
                      </td>
                      <td>
                        <div style={{ display: 'flex', gap: 8 }}>
                          <button type="button" className="btn btn-secondary btn-sm" onClick={() => startEdit(c)}>
                            ویرایش
                          </button>
                          <button
                            type="button"
                            className={c.is_active ? 'btn btn-danger btn-sm' : 'btn btn-primary btn-sm'}
                            onClick={() => handleToggleActive(c)}
                            disabled={togglingId === c.id}
                          >
                            {togglingId === c.id ? 'در حال ذخیره...' : c.is_active ? 'غیرفعال کردن' : 'فعال کردن'}
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
