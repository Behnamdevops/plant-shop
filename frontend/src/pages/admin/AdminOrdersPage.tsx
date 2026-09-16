import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getAdminOrders } from '../../api/orders'
import type { AdminOrderSummary } from '../../types/order'

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
        if (!ignore) setError(err instanceof Error ? err.message : 'Could not load orders')
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
        <h1>Admin · Orders</h1>
        <p className="page-subtitle">Inspect and manage customer orders.</p>
      </div>

      {loading && <p className="state-message">Loading orders...</p>}

      {!loading && error && (
        <>
          <p className="alert alert-error" role="alert">
            {error}
          </p>
          <button type="button" className="btn btn-secondary" onClick={reload}>
            Retry
          </button>
        </>
      )}

      {!loading && !error && orders && orders.length === 0 && (
        <p className="empty-state">No orders yet.</p>
      )}

      {!loading && !error && orders && orders.length > 0 && (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Order</th>
                <th>Customer</th>
                <th>Status</th>
                <th>Payment</th>
                <th>Shipping</th>
                <th>Total</th>
                <th>Placed</th>
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
                    <span className="status-badge">{order.status}</span>
                  </td>
                  <td>{order.payment_status}</td>
                  <td>{order.shipping_method}</td>
                  <td className="price">{order.total}</td>
                  <td>{new Date(order.created_at).toLocaleString()}</td>
                  <td>
                    <Link to={`/admin/orders/${order.id}`} className="btn btn-secondary btn-sm">
                      View
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
