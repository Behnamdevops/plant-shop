import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getOrder } from '../api/orders'
import { ApiError } from '../api/errors'
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
        if (err instanceof ApiError && err.status === 401) {
          setUnauthorized(true)
        } else if (err instanceof ApiError && err.status === 404) {
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
          <span>Payment: {order.payment_status}</span>
          <span>Shipping: {order.shipping_method}</span>
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

        <div className="checkout-summary__totals">
          <div className="checkout-summary__row">
            <span>Items subtotal</span>
            <span className="price">{order.items_subtotal}</span>
          </div>
          <div className="checkout-summary__row">
            <span>Shipping fee</span>
            <span className="price">{order.shipping_fee}</span>
          </div>
          <div className="checkout-summary__row checkout-summary__row--total">
            <span>Total</span>
            <span className="price">{order.total}</span>
          </div>
        </div>

        {(order.recipient_name || order.address_line1) && (
          <div className="order-card__delivery">
            <h2>Delivery details</h2>
            {order.recipient_name && <p>{order.recipient_name}</p>}
            {order.phone && <p>{order.phone}</p>}
            {order.address_line1 && <p>{order.address_line1}</p>}
            {order.address_line2 && <p>{order.address_line2}</p>}
            {(order.city || order.postal_code) && (
              <p>
                {order.city}
                {order.city && order.postal_code ? ', ' : ''}
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
        <p>Invalid order</p>
        <Link to="/orders">Back to orders</Link>
      </main>
    )
  }

  return <OrderDetail key={orderId} id={orderId} />
}
