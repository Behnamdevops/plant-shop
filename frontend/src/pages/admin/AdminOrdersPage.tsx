import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getAdminOrders } from '../../api/orders'
import type { AdminOrderSummary } from '../../types/order'
import { formatDateFa, formatToman } from '../../lib/format'
import { orderStatusLabel, paymentStatusLabel, shippingMethodLabel } from '../../lib/labels'

export default function AdminOrdersPage() {
  const [orders, setOrders] = useState<AdminOrderSummary[] | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [reloadKey, setReloadKey] = useState(0)

  const reload = () => {
    setLoading(true)
    setError('')
    setReloadKey((key) => key + 1)
  }

  useEffect(() => {
    let ignore = false

    getAdminOrders()
      .then((data) => {
        if (!ignore) setOrders(data)
      })
      .catch((err) => {
        if (!ignore) setError(err instanceof Error ? err.message : 'مشکلی در بارگذاری سفارش‌ها پیش آمد')
      })
      .finally(() => {
        if (!ignore) setLoading(false)
      })

    return () => {
      ignore = true
    }
  }, [reloadKey])

  return (
    <main>
      <div className="page-header">
        <h1>مدیریت · سفارش‌ها</h1>
        <p className="page-subtitle">مشاهده و مدیریت سفارش‌های مشتریان.</p>
      </div>

      {loading && <p className="state-message">در حال بارگذاری سفارش‌ها...</p>}

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

      {!loading && !error && orders && orders.length === 0 && (
        <p className="empty-state">هنوز سفارشی ثبت نشده است.</p>
      )}

      {!loading && !error && orders && orders.length > 0 && (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>سفارش</th>
                <th>مشتری</th>
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
                  <td>#{order.id}</td>
                  <td>
                    {order.customer.name}
                    <br />
                    <span className="site-header__user">{order.customer.email}</span>
                  </td>
                  <td>
                    <span className="status-badge">{orderStatusLabel(order.status)}</span>
                  </td>
                  <td>{paymentStatusLabel(order.payment_status)}</td>
                  <td>{shippingMethodLabel(order.shipping_method)}</td>
                  <td className="price">{formatToman(order.total)}</td>
                  <td>{formatDateFa(order.created_at)}</td>
                  <td>
                    <Link to={`/admin/orders/${order.id}`} className="btn btn-secondary btn-sm">
                      مشاهده
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </main>
  )
}
