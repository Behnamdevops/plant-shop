import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getOrders } from '../api/orders'
import { ApiError } from '../api/errors'
import type { OrderSummary } from '../types/order'
import { useAuth } from '../hooks/useAuth'

export default function OrdersPage() {
  const { user, loading: authLoading } = useAuth()
  const [orders, setOrders] = useState<OrderSummary[] | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [unauthorized, setUnauthorized] = useState(false)
  const [reloadKey, setReloadKey] = useState(0)

  const reload = () => {
    setLoading(true)
    setError('')
    setReloadKey((key) => key + 1)
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
          setError(err instanceof Error ? err.message : 'Could not load orders')
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
        <h1>Your orders</h1>
        <p className="state-message">Loading orders...</p>
      </main>
    )
  }

  if (!user || unauthorized) {
    return (
      <main>
        <h1>Your orders</h1>
        <p className="empty-state">
          Please <Link to="/login">log in</Link> to view your orders.
        </p>
      </main>
    )
  }

  if (loading) {
    return (
      <main>
        <h1>Your orders</h1>
        <p className="state-message">Loading orders...</p>
      </main>
    )
  }

  if (error) {
    return (
      <main>
        <h1>Your orders</h1>
        <p className="alert alert-error" role="alert">
          {error}
        </p>
        <button type="button" className="btn btn-secondary" onClick={reload}>
          Retry
        </button>
      </main>
    )
  }

  if (!orders || orders.length === 0) {
    return (
      <main>
        <h1>Your orders</h1>
        <p className="empty-state">
          You have no orders yet. <Link to="/">Browse products</Link>
        </p>
      </main>
    )
  }

  return (
    <main>
      <h1>Your orders</h1>

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Order</th>
              <th>Status</th>
              <th>Total</th>
              <th>Placed</th>
            </tr>
          </thead>
          <tbody>
            {orders.map((order) => (
              <tr key={order.id}>
                <td>
                  <Link to={`/orders/${order.id}`}>#{order.id}</Link>
                </td>
                <td>
                  <span className="status-badge">{order.status}</span>
                </td>
                <td className="price">{order.total}</td>
                <td>{new Date(order.created_at).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </main>
  )
}
