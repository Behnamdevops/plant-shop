import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getOrders } from '../api/orders'
import { requestZarinPalPayment } from '../api/payments'
import { ApiError } from '../api/errors'
import type { OrderSummary } from '../types/order'
import { useAuth } from '../hooks/useAuth'
import { formatDateFa, formatToman } from '../lib/format'
import { orderStatusLabel, paymentStatusLabel, shippingMethodLabel } from '../lib/labels'

export default function OrdersPage() {
  const { user, loading: authLoading } = useAuth()
  const [orders, setOrders] = useState<OrderSummary[] | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [unauthorized, setUnauthorized] = useState(false)
  const [reloadKey, setReloadKey] = useState(0)
  const [payingId, setPayingId] = useState<number | null>(null)
  const [payError, setPayError] = useState('')

  const reload = () => {
    setLoading(true)
    setError('')
    setReloadKey((key) => key + 1)
  }

  const handlePay = async (orderId: number) => {
    setPayError('')
    setPayingId(orderId)
    try {
      const { redirect_url } = await requestZarinPalPayment(orderId)
      window.location.assign(redirect_url)
    } catch {
      setPayError('اتصال به درگاه پرداخت زرین‌پال با مشکل مواجه شد. دوباره تلاش کنید.')
      setPayingId(null)
    }
  }

  useEffect(() => {
    if (authLoading || !user) {
      return
    }

    let ignore = false

    getOrders()
      .then((data) => {
        if (ignore) return
        setOrders(data)
        setUnauthorized(false)
      })
      .catch((err) => {
        if (ignore) return
        if (err instanceof ApiError && err.status === 401) {
          setUnauthorized(true)
        } else {
          setError(err instanceof Error ? err.message : 'مشکلی در بارگذاری سفارش‌ها پیش آمد')
        }
      })
      .finally(() => {
        if (!ignore) setLoading(false)
      })

    return () => {
      ignore = true
    }
  }, [authLoading, user, reloadKey])

  if (authLoading || (!user && loading)) {
    return (
      <main>
        <h1>سفارش‌های من</h1>
        <p className="state-message">در حال بارگذاری سفارش‌ها...</p>
      </main>
    )
  }

  if (!user || unauthorized) {
    return (
      <main>
        <h1>سفارش‌های من</h1>
        <p className="empty-state">
          برای مشاهده سفارش‌ها، <Link to="/login">وارد شوید</Link>.
        </p>
      </main>
    )
  }

  if (loading) {
    return (
      <main>
        <h1>سفارش‌های من</h1>
        <p className="state-message">در حال بارگذاری سفارش‌ها...</p>
      </main>
    )
  }

  if (error) {
    return (
      <main>
        <h1>سفارش‌های من</h1>
        <p className="alert alert-error" role="alert">
          {error}
        </p>
        <button type="button" className="btn btn-secondary" onClick={reload}>
          تلاش دوباره
        </button>
      </main>
    )
  }

  if (!orders || orders.length === 0) {
    return (
      <main>
        <h1>سفارش‌های من</h1>
        <p className="empty-state">
          هنوز سفارشی ثبت نکرده‌اید. <Link to="/">مشاهده محصولات</Link>
        </p>
      </main>
    )
  }

  // A ZarinPal order is eligible for (re)payment as long as it hasn't been
  // paid yet and hasn't reached a terminal state. "manual" orders are not
  // offered a pay button here — V1 has no gateway-driven manual flow.
  const canPay = (order: OrderSummary) =>
    order.payment_status !== 'paid' && order.status !== 'cancelled' && order.status !== 'delivered'

  return (
    <main>
      <h1>سفارش‌های من</h1>

      {payError && (
        <p className="alert alert-error" role="alert">
          {payError}
        </p>
      )}

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>سفارش</th>
              <th>وضعیت</th>
              <th>پرداخت</th>
              <th>ارسال</th>
              <th>جمع کل</th>
              <th>تاریخ ثبت</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {orders.map((order) => (
              <tr key={order.id}>
                <td>
                  <Link to={`/orders/${order.id}`}>#{order.id}</Link>
                </td>
                <td>
                  <span className="status-badge">{orderStatusLabel(order.status)}</span>
                </td>
                <td>{paymentStatusLabel(order.payment_status)}</td>
                <td>{shippingMethodLabel(order.shipping_method)}</td>
                <td className="price">{formatToman(order.total)}</td>
                <td>{formatDateFa(order.created_at)}</td>
                <td>
                  {canPay(order) && (
                    <button
                      type="button"
                      className="btn btn-primary btn-sm"
                      onClick={() => handlePay(order.id)}
                      disabled={payingId === order.id}
                    >
                      {payingId === order.id
                        ? 'در حال انتقال...'
                        : order.payment_status === 'failed'
                          ? 'تلاش مجدد برای پرداخت'
                          : 'پرداخت'}
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </main>
  )
}
