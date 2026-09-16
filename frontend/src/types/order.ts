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
