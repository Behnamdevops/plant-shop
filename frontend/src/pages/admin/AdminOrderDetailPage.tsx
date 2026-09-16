import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getAdminOrder, updateAdminOrderStatus } from '../../api/orders'
import { ApiError } from '../../api/errors'
import type { AdminOrderDetails } from '../../types/order'
import { ORDER_STATUS_TRANSITIONS } from '../../types/order'

type AdminOrderDetailProps = {
  id: number
}

function AdminOrderDetail({ id }: AdminOrderDetailProps) {
  const [order, setOrder] = useState<AdminOrderDetails | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notFound, setNotFound] = useState(false)

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
          setError(err instanceof Error ? err.message : 'Could not load order')
        }
      })
      .finally(() => {
        if (!ignore) setLoading(false)
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
        setStatusError(`Cannot change status from "${order.status}" to "${selectedStatus}".`)
      } else if (err instanceof ApiError && err.status === 400) {
        setStatusError('Invalid status.')
      } else {
        setStatusError(err instanceof Error ? err.message : 'Could not update status')
      }
    } finally {
      setSubmitting(false)
    }
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
        <Link to="/admin/orders" className="back-link">
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
        <Link to="/admin/orders" className="back-link">
          ← Back to orders
        </Link>
      </main>
    )
  }

  return (
    <main>
      <Link to="/admin/orders" className="back-link">
        ← Back to orders
      </Link>

      <div className="order-card">
        <div className="order-card__header">
          <h1>Order #{order.id}</h1>
          <span className="status-badge">{order.status}</span>
        </div>

        <div className="order-card__meta">
          <span>Customer: {order.customer.name} ({order.customer.email})</span>
          <span>Placed: {new Date(order.created_at).toLocaleString()}</span>
          <span>Updated: {new Date(order.updated_at).toLocaleString()}</span>
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
                  <td>{item.product_name}</td>
                  <td className="price">{item.unit_price}</td>
                  <td>{item.quantity}</td>
                  <td className="price">{item.subtotal}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div className="form-field" style={{ marginTop: 24 }}>
          <label htmlFor="order-status">Update status</label>
          {availableTransitions.length === 0 ? (
            <p className="empty-state">This order is in a final state and cannot be changed further.</p>
          ) : (
            <div style={{ display: 'flex', gap: 8 }}>
              <select
                id="order-status"
                value={selectedStatus}
                onChange={(event) => setSelectedStatus(event.target.value)}
                disabled={submitting}
              >
                <option value="">Select a status...</option>
                {availableTransitions.map((status) => (
                  <option key={status} value={status}>
                    {status}
                  </option>
                ))}
              </select>
              <button
                type="button"
                className="btn btn-primary btn-sm"
                onClick={handleStatusSubmit}
                disabled={submitting || !selectedStatus}
              >
                {submitting ? 'Updating...' : 'Update status'}
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
        <p>Invalid order</p>
        <Link to="/admin/orders">Back to orders</Link>
      </main>
    )
  }

  return <AdminOrderDetail key={orderId} id={orderId} />
}
