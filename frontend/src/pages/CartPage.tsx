import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { deleteCartItem, getCart, updateCartItem } from '../api/cart'
import { ApiError } from '../api/errors'
import type { Cart } from '../types/cart'
import { useAuth } from '../hooks/useAuth'

export default function CartPage() {
  const { user, loading: authLoading } = useAuth()
  const navigate = useNavigate()
  const [cart, setCart] = useState<Cart | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [unauthorized, setUnauthorized] = useState(false)
  const [pendingItemId, setPendingItemId] = useState<number | null>(null)
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

    getCart()
      .then((data) => {
        if (ignore) return
        setCart(data)
        setUnauthorized(false)
      })
      .catch((err) => {
        if (ignore) return
        if (err instanceof ApiError && err.status === 401) {
          setUnauthorized(true)
        } else {
          setError(err instanceof Error ? err.message : 'Could not load cart')
        }
      })
      .finally(() => {
        if (!ignore) setLoading(false)
      })

    return () => {
      ignore = true
    }
  }, [authLoading, user, reloadKey])

  const handleQuantityChange = async (itemId: number, quantity: number) => {
    if (quantity < 1) {
      return
    }

    setError('')
    setPendingItemId(itemId)

    try {
      await updateCartItem(itemId, quantity)
      reload()
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setUnauthorized(true)
      } else {
        setError(err instanceof Error ? err.message : 'Could not update item')
      }
    } finally {
      setPendingItemId(null)
    }
  }

  const handleDelete = async (itemId: number) => {
    setError('')
    setPendingItemId(itemId)

    try {
      await deleteCartItem(itemId)
      reload()
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setUnauthorized(true)
      } else {
        setError(err instanceof Error ? err.message : 'Could not remove item')
      }
    } finally {
      setPendingItemId(null)
    }
  }

  const handleCheckout = () => {
    navigate('/checkout')
  }

  if (authLoading || (!user && loading)) {
    return (
      <main>
        <h1>Your cart</h1>
        <p className="state-message">Loading cart...</p>
      </main>
    )
  }

  if (!user || unauthorized) {
    return (
      <main>
        <h1>Your cart</h1>
        <p className="empty-state">
          Please <Link to="/login">log in</Link> to view your cart.
        </p>
      </main>
    )
  }

  if (loading) {
    return (
      <main>
        <h1>Your cart</h1>
        <p className="state-message">Loading cart...</p>
      </main>
    )
  }

  if (error) {
    return (
      <main>
        <h1>Your cart</h1>
        <p className="alert alert-error" role="alert">
          {error}
        </p>
        <button type="button" className="btn btn-secondary" onClick={reload}>
          Retry
        </button>
      </main>
    )
  }

  if (!cart || cart.items.length === 0) {
    return (
      <main>
        <h1>Your cart</h1>
        <p className="empty-state">
          Your cart is empty. <Link to="/">Browse products</Link>
        </p>
      </main>
    )
  }

  return (
    <main>
      <h1>Your cart</h1>

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Product</th>
              <th>Unit price</th>
              <th>Quantity</th>
              <th>Subtotal</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {cart.items.map((item) => (
              <tr key={item.id}>
                <td>
                  <Link to={`/products/${item.slug}`}>{item.name}</Link>
                </td>
                <td className="price">{item.price}</td>
                <td>
                  <div className="qty-control">
                    <button
                      type="button"
                      onClick={() => handleQuantityChange(item.id, item.quantity - 1)}
                      disabled={pendingItemId === item.id || item.quantity <= 1}
                      aria-label={`Decrease quantity of ${item.name}`}
                    >
                      -
                    </button>
                    <input
                      type="number"
                      min={1}
                      value={item.quantity}
                      disabled={pendingItemId === item.id}
                      onChange={(event) => {
                        const value = Number(event.target.value)
                        if (Number.isFinite(value) && value >= 1) {
                          handleQuantityChange(item.id, value)
                        }
                      }}
                      aria-label={`Quantity of ${item.name}`}
                    />
                    <button
                      type="button"
                      onClick={() => handleQuantityChange(item.id, item.quantity + 1)}
                      disabled={pendingItemId === item.id}
                      aria-label={`Increase quantity of ${item.name}`}
                    >
                      +
                    </button>
                  </div>
                </td>
                <td className="price">{item.subtotal}</td>
                <td>
                  <button
                    type="button"
                    className="btn btn-danger btn-sm"
                    onClick={() => handleDelete(item.id)}
                    disabled={pendingItemId === item.id}
                  >
                    Remove
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="cart-summary">
        <span className="cart-summary__total">
          Total: <span className="price">{cart.total}</span>
        </span>

        {cart.items.length > 0 && (
          <button type="button" className="btn btn-primary" onClick={handleCheckout}>
            Checkout
          </button>
        )}
      </div>
    </main>
  )
}
