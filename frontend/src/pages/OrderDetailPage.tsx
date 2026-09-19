import { useEffect, useState } from 'react'
import { Link, useLocation, useParams } from 'react-router-dom'
import { cancelOrder, getOrder } from '../api/orders'
import { requestZarinPalPayment } from '../api/payments'
import { ApiError } from '../api/errors'
import type { OrderDetails } from '../types/order'
import { useAuth } from '../hooks/useAuth'
import { formatDateFa, formatToman } from '../lib/format'
import { orderStatusLabel, paymentStatusLabel, shippingMethodLabel } from '../lib/labels'

type OrderDetailProps = {
  id: number
}

function OrderDetail({ id }: OrderDetailProps) {
  const { user, loading: authLoading } = useAuth()
  const location = useLocation()
  const notice = (location.state as { notice?: string } | null)?.notice ?? ''
  const [order, setOrder] = useState<OrderDetails | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [unauthorized, setUnauthorized] = useState(false)
  const [notFound, setNotFound] = useState(false)
  const [paying, setPaying] = useState(false)
  const [payError, setPayError] = useState('')
  const [cancelling, setCancelling] = useState(false)
  const [cancelError, setCancelError] = useState('')

  useEffect(() => {
    if (authLoading || !user) {
      return
    }

    let ignore = false

    getOrder(id)
      .then((data) => {
        if (!ignore) {
          setOrder(data)
        }
      })
      .catch((err) => {
        if (ignore) return
        if (err instanceof ApiError && err.status === 401) {
          setUnauthorized(true)
        } else if (err instanceof ApiError && err.status === 404) {
          setNotFound(true)
        } else {
          setError(err instanceof Error ? err.message : 'مشکلی در بارگذاری سفارش پیش آمد')
        }
      })
      .finally(() => {
        if (!ignore) setLoading(false)
      })

    return () => {
      ignore = true
    }
  }, [authLoading, user, id])

  const handlePay = async () => {
    setPayError('')
    setPaying(true)
    try {
      const { redirect_url } = await requestZarinPalPayment(id)
      window.location.assign(redirect_url)
    } catch {
      setPayError('اتصال به درگاه پرداخت زرین‌پال با مشکل مواجه شد. دوباره تلاش کنید.')
      setPaying(false)
    }
  }

  const handleCancel = async () => {
    setCancelError('')
    setCancelling(true)
    try {
      const updated = await cancelOrder(id)
      setOrder(updated)
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setCancelError('این سفارش دیگر قابل لغو نیست.')
      } else {
        setCancelError('لغو سفارش با مشکل مواجه شد.')
      }
    } finally {
      setCancelling(false)
    }
  }

  if (authLoading || (!user && loading)) {
    return (
      <main>
        <h1>جزئیات سفارش</h1>
        <p className="state-message">در حال بارگذاری سفارش...</p>
      </main>
    )
  }

  if (!user || unauthorized) {
    return (
      <main>
        <h1>جزئیات سفارش</h1>
        <p className="empty-state">
          برای مشاهده این سفارش، <Link to="/login">وارد شوید</Link>.
        </p>
      </main>
    )
  }

  if (loading) {
    return (
      <main>
        <h1>جزئیات سفارش</h1>
        <p className="state-message">در حال بارگذاری سفارش...</p>
      </main>
    )
  }

  if (notFound) {
    return (
      <main>
        <h1>جزئیات سفارش</h1>
        <p className="empty-state">سفارش یافت نشد.</p>
        <Link to="/orders" className="back-link">
          → بازگشت به سفارش‌ها
        </Link>
      </main>
    )
  }

  if (error || !order) {
    return (
      <main>
        <h1>جزئیات سفارش</h1>
        <p className="alert alert-error" role="alert">
          {error || 'مشکلی در بارگذاری سفارش پیش آمد'}
        </p>
        <Link to="/orders" className="back-link">
          → بازگشت به سفارش‌ها
        </Link>
      </main>
    )
  }

  const canPay = order.payment_status !== 'paid' && order.status !== 'cancelled' && order.status !== 'delivered'
  const canCancel = order.payment_status !== 'paid' && (order.status === 'pending' || order.status === 'processing')

  return (
    <main>
      <Link to="/orders" className="back-link">
        → بازگشت به سفارش‌ها
      </Link>

      {notice && (
        <p className="alert alert-error" role="alert">
          {notice}
        </p>
      )}

      <div className="order-card">
        <div className="order-card__header">
          <h1>سفارش #{order.id}</h1>
          <span className="status-badge">{orderStatusLabel(order.status)}</span>
        </div>

        <div className="order-card__meta">
          <span>تاریخ ثبت: {formatDateFa(order.created_at)}</span>
          <span>وضعیت پرداخت: {paymentStatusLabel(order.payment_status)}</span>
          <span>روش ارسال: {shippingMethodLabel(order.shipping_method)}</span>
        </div>

        {(canPay || canCancel) && (
          <div className="order-card__meta" style={{ marginTop: -8 }}>
            {canPay && (
              <button type="button" className="btn btn-primary btn-sm" onClick={handlePay} disabled={paying}>
                {paying
                  ? 'در حال انتقال...'
                  : order.payment_status === 'failed'
                    ? 'تلاش مجدد برای پرداخت'
                    : 'پرداخت'}
              </button>
            )}
            {canCancel && (
              <button type="button" className="btn btn-danger btn-sm" onClick={handleCancel} disabled={cancelling}>
                {cancelling ? 'در حال لغو...' : 'لغو سفارش'}
              </button>
            )}
          </div>
        )}

        {payError && (
          <p className="alert alert-error" role="alert">
            {payError}
          </p>
        )}
        {cancelError && (
          <p className="alert alert-error" role="alert">
            {cancelError}
          </p>
        )}

        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>محصول</th>
                <th>قیمت واحد</th>
                <th>تعداد</th>
                <th>جمع جزء</th>
              </tr>
            </thead>
            <tbody>
              {order.items.map((item) => (
                <tr key={item.id}>
                  <td>
                    {item.product_slug ? (
                      <Link to={`/products/${item.product_slug}`}>{item.product_name}</Link>
                    ) : (
                      item.product_name
                    )}
                  </td>
                  <td className="price">{formatToman(item.unit_price)}</td>
                  <td>{item.quantity}</td>
                  <td className="price">{formatToman(item.subtotal)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {order.coupon_code && (
          <div className="order-card__meta">
            <span>کد تخفیف: {order.coupon_code}</span>
          </div>
        )}

        <div className="checkout-summary__totals">
          <div className="checkout-summary__row">
            <span>جمع کالاها</span>
            <span className="price">{formatToman(order.items_subtotal)}</span>
          </div>
          {order.discount_amount > 0 && (
            <div className="checkout-summary__row">
              <span>تخفیف</span>
              <span className="price">-{formatToman(order.discount_amount)}</span>
            </div>
          )}
          <div className="checkout-summary__row">
            <span>هزینه ارسال</span>
            <span className="price">{formatToman(order.shipping_fee)}</span>
          </div>
          <div className="checkout-summary__row checkout-summary__row--total">
            <span>مبلغ نهایی</span>
            <span className="price">{formatToman(order.total)}</span>
          </div>
        </div>

        {(order.recipient_name || order.address_line1) && (
          <div className="order-card__delivery">
            <h2>اطلاعات ارسال</h2>
            {order.recipient_name && <p>{order.recipient_name}</p>}
            {order.phone && <p>{order.phone}</p>}
            {order.address_line1 && <p>{order.address_line1}</p>}
            {order.address_line2 && <p>{order.address_line2}</p>}
            {(order.city || order.postal_code) && (
              <p>
                {order.city}
                {order.city && order.postal_code ? '، ' : ''}
                {order.postal_code}
              </p>
            )}
            {order.country && <p>{order.country}</p>}
          </div>
        )}
      </div>
    </main>
  )
}

export default function OrderDetailPage() {
  const { id } = useParams<{ id: string }>()
  const orderId = id ? Number(id) : NaN

  if (!id || !Number.isFinite(orderId) || orderId <= 0) {
    return (
      <main>
        <p>سفارش نامعتبر</p>
        <Link to="/orders">بازگشت به سفارش‌ها</Link>
      </main>
    )
  }

  return <OrderDetail key={orderId} id={orderId} />
}
