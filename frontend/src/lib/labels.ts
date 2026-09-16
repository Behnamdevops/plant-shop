// Centralized Persian label maps. Backend API fields and enum VALUES
// (status, payment_status, shipping_method, payment_method) always stay
// in English on the wire — these maps translate them for display only, in
// exactly one place, so no page ever hardcodes a Persian status string
// inline.
import type { OrderStatus, PaymentMethod, PaymentStatus, ShippingMethod } from '../types/order'

export const ORDER_STATUS_LABELS_FA: Record<OrderStatus, string> = {
  pending: 'در انتظار پرداخت',
  processing: 'در حال پردازش',
  shipped: 'ارسال شده',
  delivered: 'تحویل شده',
  cancelled: 'لغو شده',
}

export function orderStatusLabel(status: string): string {
  return ORDER_STATUS_LABELS_FA[status as OrderStatus] ?? status
}

export const PAYMENT_STATUS_LABELS_FA: Record<PaymentStatus, string> = {
  pending: 'پرداخت نشده',
  paid: 'پرداخت شده',
  failed: 'پرداخت ناموفق',
  refunded: 'بازگشت وجه',
}

export function paymentStatusLabel(status: string): string {
  return PAYMENT_STATUS_LABELS_FA[status as PaymentStatus] ?? status
}

export const SHIPPING_METHOD_LABELS_FA: Record<ShippingMethod, string> = {
  standard: 'ارسال عادی',
  express: 'ارسال فوری',
}

export function shippingMethodLabel(method: string): string {
  return SHIPPING_METHOD_LABELS_FA[method as ShippingMethod] ?? method
}

export const PAYMENT_METHOD_LABELS_FA: Record<PaymentMethod, string> = {
  manual: 'پرداخت دستی',
  zarinpal: 'زرین‌پال',
}

export function paymentMethodLabel(method: string): string {
  return PAYMENT_METHOD_LABELS_FA[method as PaymentMethod] ?? method
}
