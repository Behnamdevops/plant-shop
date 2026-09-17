import type { AdminPaymentReconciliation } from '../types/payment'
import { throwApiError } from './errors'

export async function getAdminPaymentReconciliations(
  beforeId?: number,
): Promise<AdminPaymentReconciliation[]> {
  const params = new URLSearchParams({ limit: '50' })
  if (beforeId !== undefined) {
    params.set('before_id', String(beforeId))
  }
  const query = params.toString()
  const response = await fetch(`/api/v1/admin/payments/reconciliation${query ? `?${query}` : ''}`, {
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}

export async function reconcileAdminPayment(attemptId: number): Promise<AdminPaymentReconciliation> {
  const response = await fetch(`/api/v1/admin/payments/${attemptId}/reconcile`, {
    method: 'POST',
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}
