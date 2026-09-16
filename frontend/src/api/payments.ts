import { throwApiError } from './errors'

// RequestZarinPalPaymentResult mirrors the backend's
// POST /orders/{id}/payments/zarinpal JSON response: a redirect URL for
// the browser to navigate to (ZarinPal's own gateway page). The backend
// never returns anything about the Merchant ID or provider internals here.
export type RequestZarinPalPaymentResult = {
  redirect_url: string
}

// requestZarinPalPayment starts a new ZarinPal payment attempt for the
// authenticated user's own order and returns the URL to redirect the
// browser to. The amount charged is always the order's persisted total —
// this call never sends an amount.
export async function requestZarinPalPayment(orderId: number): Promise<RequestZarinPalPaymentResult> {
  const response = await fetch(`/api/v1/orders/${orderId}/payments/zarinpal`, {
    method: 'POST',
    credentials: 'include',
  })

  if (!response.ok) {
    return throwApiError(response)
  }

  return response.json()
}
