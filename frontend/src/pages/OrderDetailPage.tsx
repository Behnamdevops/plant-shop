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
        <p>Loading order...</p>
      </main>
    )
  }

  if (!user || unauthorized) {
    return (
      <main>
        <h1>Order details</h1>
        <p>
          Please <Link to="/login">log in</Link> to view this order.
        </p>
      </main>
    )
  }

  if (loading) {
    return (
      <main>
        <h1>Order details</h1>
        <p>Loading order...</p>
      </main>
    )
  }

  if (notFound) {
    return (
      <main>
        <h1>Order details</h1>
        <p>Order not found.</p>
        <Link to="/orders">Back to orders</Link>
      </main>
    )
  }

  if (error || !order) {
    return (
      <main>
        <h1>Order details</h1>
        <p role="alert">{error || 'Could not load order'}</p>
        <Link to="/orders">Back to orders</Link>
      </main>
    )
  }

  return (
    <main>
      <Link to="/orders">← Back to orders</Link>

      <h1>Order #{order.id}</h1>
      <p>Status: {order.status}</p>
      <p>Placed: {new Date(order.created_at).toLocaleString()}</p>

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
              <td>{item.unit_price}</td>
              <td>{item.quantity}</td>
              <td>{item.subtotal}</td>
            </tr>
          ))}
        </tbody>
      </table>

      <p>
        <strong>Total: {order.total}</strong>
      </p>
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
