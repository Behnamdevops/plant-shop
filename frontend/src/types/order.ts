// V1 shipping methods supported by checkout, mirroring the backend's
// order.ShippingMethodStandard / order.ShippingMethodExpress constants.
export const SHIPPING_METHODS = ['standard', 'express'] as const

export type ShippingMethod = (typeof SHIPPING_METHODS)[number]

// All payment statuses supported by the backend's order.PaymentStatus*
// constants. Orders start "pending"; a verified ZarinPal payment (see
// api/payments.ts) moves an order to "paid" — see backend/internal/payment
// for the server-side verification flow.
export type PaymentStatus = 'pending' | 'paid' | 'failed' | 'refunded'

// Payment methods supported in V1: "manual" is the original placeholder
// (no gateway), "zarinpal" is set once an order has an associated ZarinPal
// payment attempt.
export type PaymentMethod = 'manual' | 'zarinpal'

// OrderSummary mirrors the backend's order.Order as returned by
// POST /orders and GET /orders. Note that user_id is tagged `json:"-"`
// on the backend, so it is never present in the response.
//
// Delivery snapshot fields are nullable because orders placed before the
// Checkout & Fulfillment V2 migration never collected this information;
// the backend returns JSON `null` for those historical orders rather than
// an empty string.
export type OrderSummary = {
  id: number
  status: string
  total: number
  created_at: string
  updated_at: string

  recipient_name: string | null
  phone: string | null
  address_line1: string | null
  address_line2: string | null
  city: string | null
  postal_code: string | null
  country: string | null

  shipping_method: ShippingMethod
  shipping_fee: number
  items_subtotal: number

  payment_status: PaymentStatus
  payment_method: PaymentMethod

  // coupon_code is null for orders that did not use a coupon (including
  // every order placed before Coupons & Discounts V1). discount_amount is
  // always an immutable snapshot taken at checkout time — never
  // recalculated from the coupon's current definition, since a coupon can
  // be edited or deactivated after an order that used it was placed.
  coupon_code: string | null
  discount_amount: number
}

// OrderItem mirrors the backend's order.Item as returned inside
// GET /orders/{id}. order_id is tagged `json:"-"` on the backend and is
// never present in the response.
export type OrderItem = {
  id: number
  product_id: number
  product_name: string
  product_slug: string
  unit_price: number
  quantity: number
  subtotal: number
}

// OrderDetails mirrors the backend's order.OrderWithItems, the response
// payload for GET /orders/{id}.
export type OrderDetails = OrderSummary & {
  items: OrderItem[]
}

// All order statuses supported by the admin order management workflow.
export const ORDER_STATUSES = ['pending', 'processing', 'shipped', 'delivered', 'cancelled'] as const

export type OrderStatus = (typeof ORDER_STATUSES)[number]

// Allowed next statuses for each current status, mirroring the backend's
// order.allowedTransitions state machine. Statuses absent as keys
// (delivered, cancelled) are terminal.
export const ORDER_STATUS_TRANSITIONS: Record<string, OrderStatus[]> = {
  pending: ['processing', 'cancelled'],
  processing: ['shipped', 'cancelled'],
  shipped: ['delivered'],
}

// Customer mirrors the backend's order.Customer: a safe, minimal identity
// snapshot (never includes password hashes or session data).
export type Customer = {
  id: number
  name: string
  email: string
}

// AdminOrderSummary mirrors the backend's order.AdminOrder, the response
// shape for GET /admin/orders. Kept separate from OrderSummary since the
// customer-facing API never exposes another user's identity.
export type AdminOrderSummary = OrderSummary & {
  customer: Customer
}

// AdminOrderDetails mirrors the backend's order.AdminOrderWithItems, the
// response payload for GET /admin/orders/{id}.
export type AdminOrderDetails = AdminOrderSummary & {
  items: OrderItem[]
}

// PaymentAttemptStatus mirrors the backend payment.Status* constants — a
// smaller state machine than order status, scoped to a single ZarinPal
// attempt.
export type PaymentAttemptStatus = 'pending' | 'paid' | 'failed' | 'reconciliation'

// PaymentAttempt mirrors the backend's payment.Attempt, as returned by
// GET /admin/orders/{id}/payments. Never includes card/bank details — only
// a ZarinPal ref_id once verified.
export type PaymentAttempt = {
  id: number
  order_id: number
  provider: string
  authority: string | null
  amount: number
  status: PaymentAttemptStatus
  ref_id: number | null
  provider_code: number | null
  created_at: string
  updated_at: string
  verified_at: string | null
}
