import type { ShippingMethod } from './order'

// CheckoutInput is the request body for POST /api/v1/orders. It contains
// only user-provided checkout information — no prices, fees, totals, or
// user id. The authenticated user id always comes from the session
// cookie, and all pricing is calculated server-side.
export type CheckoutInput = {
  recipient_name: string
  phone: string
  address_line1: string
  address_line2: string
  city: string
  postal_code: string
  country: string
  shipping_method: ShippingMethod
}

// SHIPPING_FEES mirrors backend/internal/order/shipping.go's
// ShippingFeeStandard / ShippingFeeExpress constants, in the same minor
// currency unit as product prices. This is used ONLY to show an estimated
// fee/total to the customer before they submit checkout — the server is
// always authoritative and recalculates the real fee itself; the actual
// order returned by createOrder() reflects the true, server-calculated
// values. There is no rates endpoint in V1 (no carrier integration), so if
// the backend constants ever change, update this map to match so the UX
// preview doesn't drift from what the server will actually charge.
export const SHIPPING_FEES: Record<ShippingMethod, number> = {
  standard: 500,
  express: 1500,
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
