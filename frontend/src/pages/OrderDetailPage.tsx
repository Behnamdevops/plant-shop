import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getOrder } from '../api/orders'
import type { OrderDetails } from '../types/order'
import { useAuth } from '../hooks/useAuth'

type OrderDetailProps = {
  id: number
}

function OrderDetail({ id }: OrderDetailProps) {
  const { user, loading: authLoading } = useAuth()
  const [order, setOrder] = useState<OrderDetails | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [unauthorized, setUnauthorized] = useState(false)
  const [notFound, setNotFound] = useState(false)

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
        if (err instanceof Error && err.message.includes('401')) {
          setUnauthorized(true)
        } else if (err instanceof Error && err.message.includes('404')) {
          setNotFound(true)
        } else {
          setError(err instanceof Error ? err.message : 'Could not load order')
        }
      })
      .finally(() => {
        if (!ignore) setLoading(false)
      })

    return () => {
      ignore = true
    }
  }, [authLoading, user, id])

  if (authLoading || (!user && loading)) {
    return (
      <main>
        <h1>Order details</h1>
        <p className="state-message">Loading order...</p>
      </main>
    )
  }

  if (!user || unauthorized) {
    return (
      <main>
        <h1>Order details</h1>
        <p className="empty-state">
          Please <Link to="/login">log in</Link> to view this order.
        </p>
      </main>
    )
  }

  if (loading) {
    return (
      <main>
        <h1>Order details</h1>
        <p className="state-message">Loading order...</p>
      </main>
    )
  }

  if (notFound) {
    return (
      <main>
        <h1>Order details</h1>
        <p className="empty-state">Order not found.</p>
        <Link to="/orders" className="back-link">
          ← Back to orders
        </Link>
      </main>
    )
  }

  if (error || !order) {
    return (
      <main>
        <h1>Order details</h1>
        <p className="alert alert-error" role="alert">
          {error || 'Could not load order'}
        </p>
        <Link to="/orders" className="back-link">
          ← Back to orders
        </Link>
      </main>
    )
  }

  return (
    <main>
      <Link to="/orders" className="back-link">
        ← Back to orders
      </Link>

      <div className="order-card">
        <div className="order-card__header">
          <h1>Order #{order.id}</h1>
          <span className="status-badge">{order.status}</span>
        </div>

        <div className="order-card__meta">
          <span>Placed: {new Date(order.created_at).toLocaleString()}</span>
          <span>
            Total: <span className="price">{order.total}</span>
          </span>
        </div>

        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Product</th>
                <th>Unit price</th>
                <th>Quantity</th>
                <th>Subtotal</th>
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
                  <td className="price">{item.unit_price}</td>
                  <td>{item.quantity}</td>
                  <td className="price">{item.subtotal}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
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
        <p>Invalid order</p>
        <Link to="/orders">Back to orders</Link>
      </main>
    )
  }

  return <OrderDetail key={orderId} id={orderId} />
}
