// Centralized store configuration. Values come from Vite env vars
// (VITE_* — see .env.example) so this project can be reused for a
// different store without hunting through components for hard-coded
// strings. All values have sensible defaults so local development works
// with no .env file at all.
export const storeConfig = {
  // Display name shown in the header, homepage, and document title.
  name: import.meta.env.VITE_STORE_NAME?.trim() || 'Plant Shop',

  // Currency symbol prefixed to prices (prices themselves stay in minor
  // units as returned by the backend; this only affects display).
  currencySymbol: import.meta.env.VITE_CURRENCY_SYMBOL?.trim() || '$',
}
