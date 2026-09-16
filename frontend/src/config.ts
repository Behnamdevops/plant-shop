// Centralized store configuration. Values come from Vite env vars
// (VITE_* — see .env.example) so this project can be reused for a
// different store without hunting through components for hard-coded
// strings. All values have sensible defaults so local development works
// with no .env file at all.
export const storeConfig = {
  // Display name shown in the header, homepage, and document title.
  name: import.meta.env.VITE_STORE_NAME?.trim() || 'گلفروشی',

  // Retained for backward compatibility with VITE_CURRENCY_SYMBOL, but no
  // longer used for price display: the Persian storefront always shows
  // prices in تومان via src/lib/format.ts (formatToman), never a
  // configurable symbol.
  currencySymbol: import.meta.env.VITE_CURRENCY_SYMBOL?.trim() || '$',
}
