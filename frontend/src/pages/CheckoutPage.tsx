import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { getCart } from '../api/cart'
import { createOrder } from '../api/orders'
import { ApiError } from '../api/errors'
import type { Cart } from '../types/cart'
import { emptyCheckoutInput, SHIPPING_FEES } from '../types/checkout'
import type { CheckoutInput } from '../types/checkout'
import { SHIPPING_METHODS } from '../types/order'
import type { ShippingMethod } from '../types/order'
import { useAuth } from '../hooks/useAuth'

export default function CheckoutPage() {
  const { user, loading: authLoading } = useAuth()
  const navigate = useNavigate()

  const [cart, setCart] = useState<Cart | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [unauthorized, setUnauthorized] = useState(false)

  const [form, setForm] = useState<CheckoutInput>(emptyCheckoutInput())
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState('')

  useEffect(() => {
    if (authLoading || !user) {
      return
    }

    let ignore = false

    getCart()
      .then((data) => {
        if (!ignore) setCart(data)
      })
      .catch((err) => {
        if (ignore) return
        if (err instanceof ApiError && err.status === 401) {
          setUnauthorized(true)
        } else {
          setLoadError(err instanceof Error ? err.message : 'Could not load cart')
        }
      })
      .finally(() => {
        if (!ignore) setLoading(false)
      })

    return () => {
      ignore = true
    }
  }, [authLoading, user])

  const handleChange = (field: keyof CheckoutInput) => (event: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    setForm((prev) => ({ ...prev, [field]: event.target.value }))
  }

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    setSubmitError('')
    setSubmitting(true)

    try {
      const order = await createOrder(form)
      navigate(`/orders/${order.id}`)
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setUnauthorized(true)
      } else if (err instanceof ApiError && err.status === 400) {
        setSubmitError(err.message || 'Please check the delivery details and try again.')
      } else if (err instanceof ApiError && err.status === 409) {
        setSubmitError('One or more items no longer have enough stock.')
      } else {
        setSubmitError(err instanceof Error ? err.message : 'Could not place order')
      }
    } finally {
      setSubmitting(false)
    }
  }

  if (authLoading || (!user && loading)) {
    return (
      <main>
        <h1>Checkout</h1>
        <p className="state-message">Loading checkout...</p>
      </main>
    )
  }

  if (!user || unauthorized) {
    return (
      <main>
        <h1>Checkout</h1>
        <p className="empty-state">
          Please <Link to="/login">log in</Link> to check out.
        </p>
      </main>
    )
  }

  if (loading) {
    return (
      <main>
        <h1>Checkout</h1>
        <p className="state-message">Loading checkout...</p>
      </main>
    )
  }

  if (loadError) {
    return (
      <main>
        <h1>Checkout</h1>
        <p className="alert alert-error" role="alert">
          {loadError}
        </p>
        <Link to="/cart" className="back-link">
          ← Back to cart
        </Link>
      </main>
    )
  }

  if (!cart || cart.items.length === 0) {
    return (
      <main>
        <h1>Checkout</h1>
        <p className="empty-state">
          Your cart is empty. <Link to="/">Browse products</Link>
        </p>
      </main>
    )
  }

  const shippingFee = SHIPPING_FEES[form.shipping_method]
  const itemsSubtotal = cart.total
  const estimatedTotal = itemsSubtotal + shippingFee

  return (
    <main>
      <Link to="/cart" className="back-link">
        ← Back to cart
      </Link>

      <h1>Checkout</h1>

      <div className="checkout-layout">
        <form className="form-card checkout-form" onSubmit={handleSubmit}>
          <h2>Delivery details</h2>

          <div className="form-field">
            <label htmlFor="recipient_name">Recipient name</label>
            <input
              id="recipient_name"
              type="text"
              required
              maxLength={255}
              value={form.recipient_name}
              onChange={handleChange('recipient_name')}
              disabled={submitting}
            />
          </div>

          <div className="form-field">
            <label htmlFor="phone">Phone</label>
            <input
              id="phone"
              type="tel"
              required
              maxLength={64}
              value={form.phone}
              onChange={handleChange('phone')}
              disabled={submitting}
            />
          </div>

          <div className="form-field">
            <label htmlFor="address_line1">Address line 1</label>
            <input
              id="address_line1"
              type="text"
              required
              maxLength={255}
              value={form.address_line1}
              onChange={handleChange('address_line1')}
              disabled={submitting}
            />
          </div>

          <div className="form-field">
            <label htmlFor="address_line2">Address line 2 (optional)</label>
            <input
              id="address_line2"
              type="text"
              maxLength={255}
              value={form.address_line2}
              onChange={handleChange('address_line2')}
              disabled={submitting}
            />
          </div>

          <div className="form-field">
            <label htmlFor="city">City</label>
            <input
              id="city"
              type="text"
              required
              maxLength={128}
              value={form.city}
              onChange={handleChange('city')}
              disabled={submitting}
            />
          </div>

          <div className="form-field">
            <label htmlFor="postal_code">Postal code</label>
            <input
              id="postal_code"
              type="text"
              required
              maxLength={32}
              value={form.postal_code}
              onChange={handleChange('postal_code')}
              disabled={submitting}
            />
          </div>

          <div className="form-field">
            <label htmlFor="country">Country</label>
            <input
              id="country"
              type="text"
              required
              maxLength={128}
              value={form.country}
              onChange={handleChange('country')}
              disabled={submitting}
            />
          </div>

          <div className="form-field">
            <label htmlFor="shipping_method">Shipping method</label>
            <select
              id="shipping_method"
              value={form.shipping_method}
              onChange={handleChange('shipping_method')}
              disabled={submitting}
            >
              {SHIPPING_METHODS.map((method: ShippingMethod) => (
                <option key={method} value={method}>
                  {method === 'standard' ? 'Standard' : 'Express'} — <span className="price">{SHIPPING_FEES[method]}</span>
                </option>
              ))}
            </select>
          </div>

          {submitError && (
            <p className="alert alert-error" role="alert">
              {submitError}
            </p>
          )}

          <button type="submit" className="btn btn-primary btn-block" disabled={submitting}>
            {submitting ? 'Placing order...' : 'Place order'}
          </button>
        </form>

        <div className="checkout-summary">
          <h2>Order summary</h2>

          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Product</th>
                  <th>Qty</th>
                  <th>Subtotal</th>
                </tr>
              </thead>
              <tbody>
                {cart.items.map((item) => (
                  <tr key={item.id}>
                    <td>{item.name}</td>
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
              <span className="price">{itemsSubtotal}</span>
            </div>
            <div className="checkout-summary__row">
              <span>Shipping ({form.shipping_method})</span>
              <span className="price">{shippingFee}</span>
            </div>
            <div className="checkout-summary__row checkout-summary__row--total">
              <span>Estimated total</span>
              <span className="price">{estimatedTotal}</span>
            </div>
          </div>
          <p className="page-subtitle">
            The final total is calculated by the server when you place your order.
          </p>
        </div>
      </div>
    </main>
  )
}
