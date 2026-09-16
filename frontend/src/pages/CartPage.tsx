import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { deleteCartItem, getCart, updateCartItem } from '../api/cart'
import { ApiError } from '../api/errors'
import type { Cart } from '../types/cart'
import { useAuth } from '../hooks/useAuth'
import { formatToman } from '../lib/format'

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
          setError(err instanceof Error ? err.message : 'مشکلی در بارگذاری سبد خرید پیش آمد')
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
        setError(err instanceof Error ? err.message : 'مشکلی در به‌روزرسانی کالا پیش آمد')
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
        setError(err instanceof Error ? err.message : 'مشکلی در حذف کالا پیش آمد')
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
        <h1>سبد خرید شما</h1>
        <p className="state-message">در حال بارگذاری سبد خرید...</p>
      </main>
    )
  }

  if (!user || unauthorized) {
    return (
      <main>
        <h1>سبد خرید شما</h1>
        <p className="empty-state">
          برای مشاهده سبد خرید، <Link to="/login">وارد شوید</Link>.
        </p>
      </main>
    )
  }

  if (loading) {
    return (
      <main>
        <h1>سبد خرید شما</h1>
        <p className="state-message">در حال بارگذاری سبد خرید...</p>
      </main>
    )
  }

  if (error) {
    return (
      <main>
        <h1>سبد خرید شما</h1>
        <p className="alert alert-error" role="alert">
          {error}
        </p>
        <button type="button" className="btn btn-secondary" onClick={reload}>
          تلاش دوباره
        </button>
      </main>
    )
  }

  if (!cart || cart.items.length === 0) {
    return (
      <main>
        <h1>سبد خرید شما</h1>
        <p className="empty-state">
          سبد خرید شما خالی است. <Link to="/">مشاهده محصولات</Link>
        </p>
      </main>
    )
  }

  return (
    <main>
      <h1>سبد خرید شما</h1>

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>محصول</th>
              <th>قیمت واحد</th>
              <th>تعداد</th>
              <th>جمع جزء</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {cart.items.map((item) => (
              <tr key={item.id}>
                <td>
                  <Link to={`/products/${item.slug}`}>{item.name}</Link>
                </td>
                <td className="price">{formatToman(item.price)}</td>
                <td>
                  <div className="qty-control">
                    <button
                      type="button"
                      onClick={() => handleQuantityChange(item.id, item.quantity - 1)}
                      disabled={pendingItemId === item.id || item.quantity <= 1}
                      aria-label={`کم کردن تعداد ${item.name}`}
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
                      aria-label={`تعداد ${item.name}`}
                    />
                    <button
                      type="button"
                      onClick={() => handleQuantityChange(item.id, item.quantity + 1)}
                      disabled={pendingItemId === item.id}
                      aria-label={`افزودن تعداد ${item.name}`}
                    >
                      +
                    </button>
                  </div>
                </td>
                <td className="price">{formatToman(item.subtotal)}</td>
                <td>
                  <button
                    type="button"
                    className="btn btn-danger btn-sm"
                    onClick={() => handleDelete(item.id)}
                    disabled={pendingItemId === item.id}
                  >
                    حذف
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="cart-summary">
        <span className="cart-summary__total">
          جمع کل: <span className="price">{formatToman(cart.total)}</span>
        </span>

        {cart.items.length > 0 && (
          <button type="button" className="btn btn-primary" onClick={handleCheckout}>
            ادامه به تسویه حساب
          </button>
        )}
      </div>
    </main>
  )
}
