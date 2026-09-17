import type { OrderStatus, PaymentAttemptStatus, PaymentStatus } from './order'

export const RECONCILIATION_REASONS = [
  'awaiting_verification',
  'verification_uncertain',
  'verification_rejected',
  'provider_binding_mismatch',
  'missing_authority',
  'invalid_binding',
  'manual_reconciliation_required',
  'refund_required',
  'settled',
  'payments_disabled',
] as const

export type ReconciliationReason = (typeof RECONCILIATION_REASONS)[number]

export const RECONCILIATION_LAST_OUTCOMES = [
  'verified_success',
  'definitive_rejection',
  'uncertain',
  'manual_required',
  'already_settled',
] as const

export type ReconciliationLastOutcome = (typeof RECONCILIATION_LAST_OUTCOMES)[number]

export type AdminPaymentReconciliation = {
  id: number
  order_id: number
  provider: string
  currency: string
  environment: string
  authority: string | null
  amount: number
  status: PaymentAttemptStatus
  ref_id: number | null
  provider_code: number | null
  created_at: string
  updated_at: string
  verified_at: string | null
  order_status: OrderStatus
  payment_status: PaymentStatus
  reconciliation_required: boolean
  reason: ReconciliationReason
  retryable: boolean
  last_outcome: ReconciliationLastOutcome | ''
  last_checked_at: string | null
}
