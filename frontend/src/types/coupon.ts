// Coupon discount type, mirroring the backend's coupon.DiscountType*
// constants.
export type DiscountType = 'percent' | 'fixed'

// Coupon mirrors the backend's coupon.Coupon.
export type Coupon = {
  id: number
  code: string
  discount_type: DiscountType
  value: number
  min_order_amount: number
  usage_limit: number | null
  per_user_limit: number | null
  starts_at: string | null
  ends_at: string | null
  is_active: boolean
  created_at: string
  updated_at: string
}

// AdminCoupon mirrors the backend's coupon.AdminCoupon: a Coupon plus its
// current active (unreleased) redemption count.
export type AdminCoupon = Coupon & {
  usage_count: number
}

// CouponPreview mirrors the backend's coupon.Preview, the response payload
// for POST /api/v1/coupons/preview. It is advisory only — order creation
// always revalidates/recalculates everything server-side; never trust this
// response as authoritative when submitting the final order.
export type CouponPreview = {
  code: string
  discount_type: DiscountType
  items_subtotal: number
  discount_amount: number
  discounted_items_subtotal: number
}

// CouponInput is the admin create/update request body. All monetary
// fields are canonical IRR integers, same convention as products.price /
// orders.total elsewhere in the app.
export type CouponInput = {
  code: string
  discount_type: DiscountType
  value: number
  min_order_amount: number
  usage_limit: number | null
  per_user_limit: number | null
  starts_at: string | null
  ends_at: string | null
  is_active: boolean
}
