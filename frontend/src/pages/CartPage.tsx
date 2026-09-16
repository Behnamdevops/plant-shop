import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { deleteCartItem, getCart, updateCartItem } from '../api/cart'
import type { Cart } from '../types/cart'
import { useAuth } from '../hooks/useAuth'

export default function CartPage() {
  const { user, loading: authLoading } = useAuth()
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
        if (err instanceof Error && err.message.includes('401')) {
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
      if (err instanceof Error && err.message.includes('401')) {
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
      if (err instanceof Error && err.message.includes('401')) {
        setUnauthorized(true)
      } else {
        setError(err instanceof Error ? err.message : 'Could not remove item')
      }
    } finally {
      setPendingItemId(null)
    }
  }

  if (authLoading || (!user && loading)) {
    return (
      <main>
        <h1>Your cart</h1>
        <p>Loading cart...</p>
      </main>
    )
  }

  if (!user || unauthorized) {
    return (
      <main>
        <h1>Your cart</h1>
        <p>
          Please <Link to="/login">log in</Link> to view your cart.
        </p>
      </main>
    )
  }

  if (loading) {
    return (
      <main>
        <h1>Your cart</h1>
        <p>Loading cart...</p>
      </main>
    )
  }

  if (error) {
    return (
      <main>
        <h1>Your cart</h1>
        <p role="alert">{error}</p>
        <button type="button" onClick={reload}>
          Retry
        </button>
      </main>
    )
  }

  if (!cart || cart.items.length === 0) {
    return (
      <main>
        <h1>Your cart</h1>
        <p>Your cart is empty.</p>
        <Link to="/">Browse products</Link>
      </main>
    )
  }

  return (
    <main>
      <h1>Your cart</h1>

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
              <td>{item.price}</td>
              <td>
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
                  style={{ width: '3em', textAlign: 'center' }}
                />
                <button
                  type="button"
                  onClick={() => handleQuantityChange(item.id, item.quantity + 1)}
                  disabled={pendingItemId === item.id}
                  aria-label={`Increase quantity of ${item.name}`}
                >
                  +
                </button>
              </td>
              <td>{item.subtotal}</td>
              <td>
                <button
                  type="button"
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

      <p>
        <strong>Total: {cart.total}</strong>
      </p>
    </main>
  )
}
