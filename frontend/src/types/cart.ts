export type CartItem = {
  id: number
  product_id: number
  name: string
  slug: string
  price: number
  quantity: number
  subtotal: number
}

export type Cart = {
  items: CartItem[]
  total: number
}
