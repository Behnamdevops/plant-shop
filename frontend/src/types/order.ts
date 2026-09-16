// OrderSummary mirrors the backend's order.Order as returned by
// POST /orders and GET /orders. Note that user_id is tagged `json:"-"`
// on the backend, so it is never present in the response.
export type OrderSummary = {
  id: number
  status: string
  total: number
  created_at: string
  updated_at: string
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
