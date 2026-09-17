// Centralized Persian label maps. Backend API fields and enum VALUES
// (status, payment_status, shipping_method, payment_method) always stay
// in English on the wire — these maps translate them for display only, in
// exactly one place, so no page ever hardcodes a Persian status string
// inline.
import type { ReconciliationLastOutcome, ReconciliationReason } from '../types/payment'
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

export const RECONCILIATION_REASON_LABELS_FA: Record<ReconciliationReason, string> = {
  awaiting_verification: 'در انتظار تأیید پرداخت',
  verification_uncertain: 'نتیجه تأیید نامشخص',
  verification_rejected: 'پرداخت رد شده',
  provider_binding_mismatch: 'عدم تطابق اتصال با درگاه',
  missing_authority: 'شناسه تراکنش (Authority) ثبت نشده',
  invalid_binding: 'اتصال نامعتبر',
  manual_reconciliation_required: 'نیازمند مغایرت‌گیری دستی',
  refund_required: 'بازگشت وجه لازم است',
  settled: 'تسویه شده',
  payments_disabled: 'پرداخت غیرفعال',
}

export function reconciliationReasonLabel(reason: string): string {
  return RECONCILIATION_REASON_LABELS_FA[reason as ReconciliationReason] ?? reason
}

export const RECONCILIATION_LAST_OUTCOME_LABELS_FA: Record<ReconciliationLastOutcome, string> = {
  verified_success: 'تأیید موفق',
  definitive_rejection: 'رد قطعی',
  uncertain: 'نامشخص',
  manual_required: 'نیازمند بررسی دستی',
  already_settled: 'قبلاً تسویه شده',
}

export function reconciliationLastOutcomeLabel(outcome: string): string {
  return RECONCILIATION_LAST_OUTCOME_LABELS_FA[outcome as ReconciliationLastOutcome] ?? (outcome || '—')
}

export function paymentAttemptStatusLabel(status: string): string {
  const labels: Record<string, string> = {
    pending: 'در انتظار تأیید',
    paid: 'پرداخت شده',
    failed: 'ناموفق',
    reconciliation: 'نیازمند مغایرت‌گیری',
  }
  return labels[status] ?? status
}
