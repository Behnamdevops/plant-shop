import type { ShippingMethod } from './order'

// CheckoutInput is the request body for POST /api/v1/orders. It contains
// only user-provided checkout information — no prices, fees, totals, or
// user id. The authenticated user id always comes from the session
// cookie, and all pricing is calculated server-side.
// coupon_code is optional. Only the code itself is ever sent — the
// backend never accepts discount_amount, coupon value, or a final total
// from the client; everything is recalculated server-side inside the
// checkout transaction.
export type CheckoutInput = {
  recipient_name: string
  phone: string
  address_line1: string
  address_line2: string
  city: string
  postal_code: string
  country: string
  shipping_method: ShippingMethod
  coupon_code?: string
}

export type ShippingRates = {
  standard: number
  express: number
  free_shipping_threshold: number
  configured: boolean
  payments_enabled: boolean
}
export async function getShippingRates(): Promise<ShippingRates> {
  const response = await fetch('/api/v1/shipping')
  if (!response.ok) throw new Error('دریافت هزینه ارسال انجام نشد.')
  return response.json()
}

export function emptyCheckoutInput(): CheckoutInput {
  return {
    recipient_name: '',
    phone: '',
    address_line1: '',
    address_line2: '',
    city: '',
    postal_code: '',
    // Iran-only storefront V1: default the country field to ایران for UX.
    // The backend does not hard-code this — it's just a sensible default
    // in the form; the field is still a free-text snapshot on the server.
    country: 'ایران',
    shipping_method: 'standard',
  }
}
