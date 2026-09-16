import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getAdminOrder, getAdminOrderPayments, updateAdminOrderStatus } from '../../api/orders'
import { ApiError } from '../../api/errors'
import type { AdminOrderDetails, PaymentAttempt } from '../../types/order'
import { ORDER_STATUS_TRANSITIONS } from '../../types/order'
import { formatDateFa, formatToman } from '../../lib/format'
import { orderStatusLabel, paymentMethodLabel, paymentStatusLabel, shippingMethodLabel } from '../../lib/labels'

type AdminOrderDetailProps = {
  id: number
}

function AdminOrderDetail({ id }: AdminOrderDetailProps) {
  const [order, setOrder] = useState<AdminOrderDetails | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notFound, setNotFound] = useState(false)
  const [payments, setPayments] = useState<PaymentAttempt[]>([])

  const [selectedStatus, setSelectedStatus] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [statusError, setStatusError] = useState('')

  useEffect(() => {
    let ignore = false

    getAdminOrder(id)
      .then((data) => {
        if (!ignore) {
          setOrder(data)
        }
      })
      .catch((err) => {
        if (ignore) return
        if (err instanceof ApiError && err.status === 404) {
          setNotFound(true)
        } else {
          setError(err instanceof Error ? err.message : 'مشکلی در بارگذاری سفارش پیش آمد')
        }
      })
      .finally(() => {
        if (!ignore) setLoading(false)
      })

    // Payment attempts are ZarinPal-specific and only exist for orders
    // paid (or attempted) via the gateway; a failure here (e.g. no
    // attempts yet) is not shown as a page-level error — the ref_id
    // section simply stays empty.
    getAdminOrderPayments(id)
      .then((data) => {
        if (!ignore) setPayments(data)
      })
      .catch(() => {
        /* non-critical */
      })

    return () => {
      ignore = true
    }
  }, [id])

  const availableTransitions = order ? ORDER_STATUS_TRANSITIONS[order.status] ?? [] : []

  const handleStatusSubmit = async () => {
    if (!order || !selectedStatus) {
      return
    }

    setStatusError('')
    setSubmitting(true)

    try {
      // Not optimistic: only apply the new state once the server confirms
      // the transition, so a rejected transition (e.g. 409) never leaves
      // the UI showing a status that was never actually persisted.
      const updated = await updateAdminOrderStatus(order.id, selectedStatus)
      setOrder(updated)
      setSelectedStatus('')
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        // The backend returns 409 both for a plain invalid state-machine
        // transition and for the ZarinPal "must be paid first" gate — the
        // response body text differs, but we show one clear Persian
        // message covering both cases rather than parsing English text.
        setStatusError(
          `تغییر وضعیت از «${orderStatusLabel(order.status)}» به «${orderStatusLabel(selectedStatus)}» ممکن نیست. اگر این سفارش با زرین‌پال پرداخت می‌شود، ابتدا باید پرداخت آن تأیید شده باشد.`,
        )
      } else if (err instanceof ApiError && err.status === 400) {
        setStatusError('وضعیت نامعتبر است.')
      } else {
        setStatusError('به‌روزرسانی وضعیت با مشکل مواجه شد.')
      }
    } finally {
      setSubmitting(false)
    }
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
        <Link to="/admin/orders" className="back-link">
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
        <Link to="/admin/orders" className="back-link">
          → بازگشت به سفارش‌ها
        </Link>
      </main>
    )
  }

  const verifiedPayments = payments.filter((p) => p.status === 'paid')

  return (
    <main>
      <Link to="/admin/orders" className="back-link">
        → بازگشت به سفارش‌ها
      </Link>

      <div className="order-card">
        <div className="order-card__header">
          <h1>سفارش #{order.id}</h1>
          <span className="status-badge">{orderStatusLabel(order.status)}</span>
        </div>

        <div className="order-card__meta">
          <span>مشتری: {order.customer.name} ({order.customer.email})</span>
          <span>تاریخ ثبت: {formatDateFa(order.created_at)}</span>
          <span>آخرین به‌روزرسانی: {formatDateFa(order.updated_at)}</span>
          <span>
            پرداخت: {paymentStatusLabel(order.payment_status)} ({paymentMethodLabel(order.payment_method)})
          </span>
          <span>ارسال: {shippingMethodLabel(order.shipping_method)}</span>
        </div>

        {verifiedPayments.length > 0 && (
          <div className="order-card__meta">
            <span>
              شناسه پیگیری زرین‌پال: {verifiedPayments[0].ref_id ?? '—'}
            </span>
          </div>
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
                  <td>{item.product_name}</td>
                  <td className="price">{formatToman(item.unit_price)}</td>
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
            <span className="price">{formatToman(order.items_subtotal)}</span>
          </div>
          <div className="checkout-summary__row">
            <span>هزینه ارسال</span>
            <span className="price">{formatToman(order.shipping_fee)}</span>
          </div>
          <div className="checkout-summary__row checkout-summary__row--total">
            <span>جمع کل</span>
            <span className="price">{formatToman(order.total)}</span>
          </div>
        </div>

        {(order.recipient_name || order.address_line1) && (
          <div className="order-card__delivery">
            <h2>آدرس ارسال</h2>
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

        <div className="form-field" style={{ marginTop: 24 }}>
          <label htmlFor="order-status">به‌روزرسانی وضعیت</label>
          {availableTransitions.length === 0 ? (
            <p className="empty-state">این سفارش در وضعیت نهایی است و دیگر قابل تغییر نیست.</p>
          ) : (
            <div style={{ display: 'flex', gap: 8 }}>
              <select
                id="order-status"
                value={selectedStatus}
                onChange={(event) => setSelectedStatus(event.target.value)}
                disabled={submitting}
              >
                <option value="">انتخاب وضعیت...</option>
                {availableTransitions.map((status) => (
                  <option key={status} value={status}>
                    {orderStatusLabel(status)}
                  </option>
                ))}
              </select>
              <button
                type="button"
                className="btn btn-primary btn-sm"
                onClick={handleStatusSubmit}
                disabled={submitting || !selectedStatus}
              >
                {submitting ? 'در حال به‌روزرسانی...' : 'به‌روزرسانی وضعیت'}
              </button>
            </div>
          )}
          {statusError && (
            <p className="alert alert-error" role="alert">
              {statusError}
            </p>
          )}
        </div>
      </div>
    </main>
  )
}

export default function AdminOrderDetailPage() {
  const { id } = useParams<{ id: string }>()
  const orderId = id ? Number(id) : NaN

  if (!id || !Number.isFinite(orderId) || orderId <= 0) {
    return (
      <main>
        <p>سفارش نامعتبر</p>
        <Link to="/admin/orders">بازگشت به سفارش‌ها</Link>
      </main>
    )
  }

  return <AdminOrderDetail key={orderId} id={orderId} />
}
