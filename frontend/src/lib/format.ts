// Centralized Persian formatting utilities. Never scatter /10 or *10
// currency conversion, or ad-hoc date formatting, across pages — always go
// through these two functions.

// CURRENCY RULE: the backend/database canonical monetary unit is the Rial
// (IRR), stored as an integer (see products.price, orders.total, etc. in
// backend/internal/*/model.go). The Persian storefront always displays
// prices in Toman, which is exactly Rial / 10. This conversion happens
// ONLY here — every page imports formatToman instead of doing its own
// arithmetic.
//
// NOTE ON EXISTING DEV DATA: this codebase's existing seed/dev product
// prices (e.g. small integers used in tests, or whatever was seeded before
// this change) were never guaranteed to represent realistic Iranian Rial
// amounts — they were arbitrary minor-unit values from the original
// English-language/generic-currency V1. This change does not rescale or
// touch any existing database row; it only changes how already-stored
// integers are displayed (divided by 10 and labeled تومان). If your
// existing dev rows show unrealistic Toman amounts, that reflects
// pre-existing arbitrary seed data, not a bug in this conversion — update
// the underlying Rial values in the database if you want realistic demo
// prices.
const tomanFormatter = new Intl.NumberFormat('fa-IR', { maximumFractionDigits: 0 })

// formatToman converts a canonical Rial integer amount to a Persian-
// formatted Toman string, e.g. 250000 (Rial) -> "۲۵٬۰۰۰ تومان".
export function formatToman(amountRial: number): string {
  const toman = Math.round(amountRial / 10)
  return `${tomanFormatter.format(toman)} تومان`
}

// formatDateFa formats an ISO timestamp (as returned by the backend, e.g.
// order.created_at) into a readable Persian date/time string using the
// browser's Intl API with the fa-IR locale (Persian/Jalali calendar).
export function formatDateFa(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) {
    return ''
  }
  return new Intl.DateTimeFormat('fa-IR', {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}
