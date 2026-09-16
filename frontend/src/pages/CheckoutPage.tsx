import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { getCart } from '../api/cart'
import { createOrder } from '../api/orders'
import { requestZarinPalPayment } from '../api/payments'
import { ApiError } from '../api/errors'
import type { Cart } from '../types/cart'
import { emptyCheckoutInput, SHIPPING_FEES } from '../types/checkout'
import type { CheckoutInput } from '../types/checkout'
import { SHIPPING_METHODS } from '../types/order'
import type { ShippingMethod } from '../types/order'
import { useAuth } from '../hooks/useAuth'
import { formatToman } from '../lib/format'
import { shippingMethodLabel } from '../lib/labels'

export default function CheckoutPage() {
  const { user, loading: authLoading } = useAuth()
  const navigate = useNavigate()

  const [cart, setCart] = useState<Cart | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [unauthorized, setUnauthorized] = useState(false)

  const [form, setForm] = useState<CheckoutInput>(emptyCheckoutInput())
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState('')
  // redirecting is set once the order was created successfully and we're
  // waiting on the ZarinPal payment-request call before navigating the
  // browser away to the gateway — kept separate from `submitting` so the
  // button/message can say something more specific ("در حال انتقال به
  // درگاه پرداخت...") than the generic "در حال ثبت سفارش...".
  const [redirecting, setRedirecting] = useState(false)

  useEffect(() => {
    if (authLoading || !user) {
      return
    }

    let ignore = false

    getCart()
      .then((data) => {
        if (!ignore) setCart(data)
      })
      .catch((err) => {
        if (ignore) return
        if (err instanceof ApiError && err.status === 401) {
          setUnauthorized(true)
        } else {
          setLoadError(err instanceof Error ? err.message : 'مشکلی در بارگذاری سبد خرید پیش آمد')
        }
      })
      .finally(() => {
        if (!ignore) setLoading(false)
      })

    return () => {
      ignore = true
    }
  }, [authLoading, user])

  const handleChange = (field: keyof CheckoutInput) => (event: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    setForm((prev) => ({ ...prev, [field]: event.target.value }))
  }

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    setSubmitError('')
    setSubmitting(true)

    // Step 1: create the order. The order is created with payment_status
    // "pending" — it is never shown to the customer as paid at this point.
    let orderId: number
    try {
      const order = await createOrder(form)
      orderId = order.id
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setUnauthorized(true)
      } else if (err instanceof ApiError && err.status === 400) {
        setSubmitError(err.message || 'لطفاً اطلاعات ارسال را بررسی و دوباره تلاش کنید.')
      } else if (err instanceof ApiError && err.status === 409) {
        setSubmitError('موجودی برخی از کالاها کافی نیست.')
      } else {
        setSubmitError('ثبت سفارش با مشکل مواجه شد.')
      }
      setSubmitting(false)
      return
    }

    // Step 2: the order exists but is unpaid — immediately request a
    // ZarinPal payment for it and redirect the browser to the gateway.
    // If this step fails (provider unavailable, network error), the order
    // itself is safe and unaffected: the customer can retry payment from
    // the order detail page at any time (see OrderDetailPage/OrdersPage).
    setSubmitting(false)
    setRedirecting(true)
    try {
      const { redirect_url } = await requestZarinPalPayment(orderId)
      window.location.assign(redirect_url)
    } catch {
      // Do not show a raw English provider error; the order is safely
      // pending and payable again from its detail page.
      navigate(`/orders/${orderId}`, {
        state: {
          notice:
            'سفارش شما ثبت شد، اما اتصال به درگاه پرداخت زرین‌پال با مشکل مواجه شد. می‌توانید دوباره تلاش کنید.',
        },
      })
    }
  }

  if (authLoading || (!user && loading)) {
    return (
      <main>
        <h1>تسویه حساب</h1>
        <p className="state-message">در حال بارگذاری...</p>
      </main>
    )
  }

  if (!user || unauthorized) {
    return (
      <main>
        <h1>تسویه حساب</h1>
        <p className="empty-state">
          برای تسویه حساب، <Link to="/login">وارد شوید</Link>.
        </p>
      </main>
    )
  }

  if (loading) {
    return (
      <main>
        <h1>تسویه حساب</h1>
        <p className="state-message">در حال بارگذاری...</p>
      </main>
    )
  }

  if (loadError) {
    return (
      <main>
        <h1>تسویه حساب</h1>
        <p className="alert alert-error" role="alert">
          {loadError}
        </p>
        <Link to="/cart" className="back-link">
          → بازگشت به سبد خرید
        </Link>
      </main>
    )
  }

  if (!cart || cart.items.length === 0) {
    return (
      <main>
        <h1>تسویه حساب</h1>
        <p className="empty-state">
          سبد خرید شما خالی است. <Link to="/">مشاهده محصولات</Link>
        </p>
      </main>
    )
  }

  const shippingFee = SHIPPING_FEES[form.shipping_method]
  const itemsSubtotal = cart.total
  const estimatedTotal = itemsSubtotal + shippingFee

  const busy = submitting || redirecting

  return (
    <main>
      <Link to="/cart" className="back-link">
        → بازگشت به سبد خرید
      </Link>

      <h1>تسویه حساب</h1>

      <div className="checkout-layout">
        <form className="form-card checkout-form" onSubmit={handleSubmit}>
          <h2>اطلاعات ارسال</h2>

          <div className="form-field">
            <label htmlFor="recipient_name">نام گیرنده</label>
            <input
              id="recipient_name"
              type="text"
              required
              maxLength={255}
              value={form.recipient_name}
              onChange={handleChange('recipient_name')}
              disabled={busy}
            />
          </div>

          <div className="form-field">
            <label htmlFor="phone">شماره موبایل</label>
            <input
              id="phone"
              type="tel"
              required
              maxLength={64}
              placeholder="09xxxxxxxxx"
              inputMode="tel"
              value={form.phone}
              onChange={handleChange('phone')}
              disabled={busy}
            />
          </div>

          <div className="form-field">
            <label htmlFor="address_line1">آدرس (خط اول)</label>
            <input
              id="address_line1"
              type="text"
              required
              maxLength={255}
              value={form.address_line1}
              onChange={handleChange('address_line1')}
              disabled={busy}
            />
          </div>

          <div className="form-field">
            <label htmlFor="address_line2">آدرس (خط دوم - اختیاری)</label>
            <input
              id="address_line2"
              type="text"
              maxLength={255}
              value={form.address_line2}
              onChange={handleChange('address_line2')}
              disabled={busy}
            />
          </div>

          <div className="form-field">
            <label htmlFor="city">شهر</label>
            <input
              id="city"
              type="text"
              required
              maxLength={128}
              value={form.city}
              onChange={handleChange('city')}
              disabled={busy}
            />
          </div>

          <div className="form-field">
            <label htmlFor="postal_code">کد پستی</label>
            <input
              id="postal_code"
              type="text"
              required
              maxLength={32}
              inputMode="numeric"
              value={form.postal_code}
              onChange={handleChange('postal_code')}
              disabled={busy}
            />
          </div>

          <div className="form-field">
            <label htmlFor="country">کشور</label>
            <input
              id="country"
              type="text"
              required
              maxLength={128}
              value={form.country}
              onChange={handleChange('country')}
              disabled={busy}
            />
          </div>

          <div className="form-field">
            <label htmlFor="shipping_method">روش ارسال</label>
            <select
              id="shipping_method"
              value={form.shipping_method}
              onChange={handleChange('shipping_method')}
              disabled={busy}
            >
              {SHIPPING_METHODS.map((method: ShippingMethod) => (
                <option key={method} value={method}>
                  {shippingMethodLabel(method)} — {formatToman(SHIPPING_FEES[method])}
                </option>
              ))}
            </select>
          </div>

          {submitError && (
            <p className="alert alert-error" role="alert">
              {submitError}
            </p>
          )}

          <button type="submit" className="btn btn-primary btn-block" disabled={busy}>
            {redirecting
              ? 'در حال انتقال به درگاه پرداخت زرین‌پال...'
              : submitting
                ? 'در حال ثبت سفارش...'
                : 'ثبت سفارش و پرداخت'}
          </button>
        </form>

        <div className="checkout-summary">
          <h2>خلاصه سفارش</h2>

          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>محصول</th>
                  <th>تعداد</th>
                  <th>جمع جزء</th>
                </tr>
              </thead>
              <tbody>
                {cart.items.map((item) => (
                  <tr key={item.id}>
                    <td>{item.name}</td>
                    <td>{item.quantity}</td>
                    <td className="price">{formatToman(item.subtotal)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="checkout-summary__totals">
            <div className="checkout-summary__row">
              <span>جمع جزء کالاها</span>
              <span className="price">{formatToman(itemsSubtotal)}</span>
            </div>
            <div className="checkout-summary__row">
              <span>هزینه ارسال ({shippingMethodLabel(form.shipping_method)})</span>
              <span className="price">{formatToman(shippingFee)}</span>
            </div>
            <div className="checkout-summary__row checkout-summary__row--total">
              <span>جمع کل (تخمینی)</span>
              <span className="price">{formatToman(estimatedTotal)}</span>
            </div>
          </div>
          <p className="page-subtitle">
            جمع نهایی هنگام ثبت سفارش توسط سرور محاسبه می‌شود. پس از ثبت سفارش، به درگاه پرداخت زرین‌پال منتقل خواهید شد.
          </p>
        </div>
      </div>
    </main>
  )
}
