import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { getOrder } from '../api/orders'
import type { OrderDetails } from '../types/order'
import { formatToman } from '../lib/format'
import { paymentStatusLabel } from '../lib/labels'

// PaymentResultPage is where the backend's ZarinPal callback redirects the
// browser to (see backend/internal/payment/handler.go's redirectResult).
// The query string's `outcome` is only ever used as an initial hint for
// the loading message — it is NEVER trusted as the source of truth. The
// actual payment/order state shown here always comes from re-fetching the
// order from our own backend, since that is the only place the payment
// was actually, server-to-server, verified.
export default function PaymentResultPage() {
  const [params] = useSearchParams()
  const orderIdParam = params.get('order_id')
  const orderId = orderIdParam ? Number(orderIdParam) : NaN
  const hasValidOrderId = Number.isFinite(orderId) && orderId > 0

  const [order, setOrder] = useState<OrderDetails | null>(null)
  const [loading, setLoading] = useState(hasValidOrderId)
  const [loadError, setLoadError] = useState('')

  useEffect(() => {
    if (!hasValidOrderId) {
      return
    }

    let ignore = false
    getOrder(orderId)
      .then((data) => {
        if (!ignore) setOrder(data)
      })
      .catch(() => {
        if (!ignore) setLoadError('مشکلی در بررسی وضعیت سفارش پیش آمد.')
      })
      .finally(() => {
        if (!ignore) setLoading(false)
      })

    return () => {
      ignore = true
    }
  }, [hasValidOrderId, orderId])

  if (!hasValidOrderId) {
    return (
      <main>
        <h1>وضعیت پرداخت نامشخص</h1>
        <p className="empty-state">
          اطلاعات سفارش در دسترس نیست. برای بررسی وضعیت سفارش‌های خود به{' '}
          <Link to="/orders">سفارش‌های من</Link> مراجعه کنید.
        </p>
      </main>
    )
  }

  if (loading) {
    return (
      <main>
        <h1>در حال بررسی وضعیت پرداخت...</h1>
        <p className="state-message">لطفاً چند لحظه صبر کنید.</p>
      </main>
    )
  }

  if (loadError || !order) {
    return (
      <main>
        <h1>وضعیت پرداخت نامشخص</h1>
        <p className="alert alert-error" role="alert">
          {loadError || 'سفارش یافت نشد.'}
        </p>
        <Link to="/orders" className="back-link">
          → بازگشت به سفارش‌ها
        </Link>
      </main>
    )
  }

  const isPaid = order.payment_status === 'paid'
  const isFailed = order.payment_status === 'failed'

  return (
    <main>
      <div className="order-card" style={{ textAlign: 'center' }}>
        {isPaid && (
          <>
            <h1 style={{ color: 'var(--success)' }}>پرداخت موفق</h1>
            <p className="alert alert-success" role="status">
              پرداخت سفارش شما با موفقیت تأیید شد.
            </p>
          </>
        )}
        {isFailed && (
          <>
            <h1 style={{ color: 'var(--danger)' }}>پرداخت ناموفق</h1>
            <p className="alert alert-error" role="alert">
              پرداخت این سفارش ناموفق بود یا توسط شما لغو شد. می‌توانید دوباره تلاش کنید.
            </p>
          </>
        )}
        {!isPaid && !isFailed && (
          <>
            <h1>وضعیت پرداخت نامشخص</h1>
            <p className="alert alert-error" role="alert">
              وضعیت پرداخت این سفارش هنوز مشخص نیست. لطفاً چند لحظه دیگر دوباره صفحه سفارش را بررسی کنید.
            </p>
          </>
        )}

        <div className="order-card__meta" style={{ justifyContent: 'center', marginTop: 16 }}>
          <span>شماره سفارش: #{order.id}</span>
          <span>مبلغ نهایی: {formatToman(order.total)}</span>
          <span>وضعیت پرداخت: {paymentStatusLabel(order.payment_status)}</span>
        </div>

        <div style={{ marginTop: 20 }}>
          <Link to={`/orders/${order.id}`} className="btn btn-primary">
            مشاهده جزئیات سفارش
          </Link>
        </div>
      </div>
    </main>
  )
}
