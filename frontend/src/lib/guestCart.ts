import type { Product } from "../types/product";
import type { Cart } from "../types/cart";
import { getProduct } from "../api/products";
import { throwApiError } from "../api/errors";
type Entry = { product: Product; quantity: number };
type State = { key: string; items: Entry[] };
const KEY = "plantshop.guest-cart.v1";
export function cartChanged() {
  window.dispatchEvent(new Event("cart-changed"));
}
function read(): State {
  try {
    const value = JSON.parse(
      localStorage.getItem(KEY) || "null",
    ) as State | null;
    if (value && typeof value.key === "string" && Array.isArray(value.items))
      return {
        key: value.key,
        items: value.items
          .filter(
            (x) =>
              x?.product &&
              Number.isSafeInteger(x.product.id) &&
              x.product.id > 0 &&
              Number.isInteger(x.quantity) &&
              x.quantity > 0 &&
              x.quantity <= 1000,
          )
          .slice(0, 100),
      };
  } catch {
    /* Ignore malformed old state. */
  }
  return { key: crypto.randomUUID(), items: [] };
}
function write(items: Entry[]) {
  localStorage.setItem(
    KEY,
    JSON.stringify({ key: crypto.randomUUID(), items }),
  );
  cartChanged();
}
export function addGuestItem(product: Product, quantity: number) {
  const state = read();
  const item = state.items.find((x) => x.product.id === product.id);
  const total = (item?.quantity || 0) + quantity;
  if (!Number.isInteger(quantity) || quantity < 1 || total > product.stock)
    throw new Error("موجودی این محصول کافی نیست.");
  if (item) {
    item.quantity = total;
    item.product = product;
  } else {
    if (state.items.length >= 100)
      throw new Error("حداکثر ۱۰۰ محصول در سبد قابل ذخیره است.");
    state.items.push({ product, quantity });
  }
  write(state.items);
  return {
    id: -product.id,
    product_id: product.id,
    kind: product.details?.kind,
    quantity: total,
    created_at: "",
    updated_at: "",
  };
}
export async function getGuestCart(): Promise<Cart> {
  const items = await Promise.all(
    read().items.map(async (entry) => {
      let product = entry.product;
      try {
        product = await getProduct(product.slug);
      } catch {
        /* Checkout always refreshes price and availability on the server. */
      }
      return {
        id: -product.id,
        product_id: product.id,
        kind: product.details?.kind,
        name: product.name,
        slug: product.slug,
        price: product.price,
        quantity: entry.quantity,
        subtotal: product.price * entry.quantity,
      };
    }),
  );
  return { items, total: items.reduce((sum, item) => sum + item.subtotal, 0) };
}
export function updateGuestItem(id: number, quantity: number) {
  if (!Number.isInteger(quantity) || quantity < 1 || quantity > 1000)
    throw new Error("تعداد معتبر نیست.");
  const state = read();
  const item = state.items.find((x) => x.product.id === -id);
  if (!item) throw new Error("محصول پیدا نشد.");
  item.quantity = quantity;
  write(state.items);
}
export function deleteGuestItem(id: number) {
  write(read().items.filter((x) => x.product.id !== -id));
}
let merging: Promise<void> | undefined;
export function mergeGuestCart(): Promise<void> {
  if (merging) return merging;
  const state = read();
  if (!state.items.length) return Promise.resolve();
  merging = (async () => {
    const response = await fetch("/api/v1/cart/merge", {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        merge_key: state.key,
        items: state.items.map((x) => ({
          product_id: x.product.id,
          quantity: x.quantity,
        })),
      }),
    });
    if (!response.ok) await throwApiError(response);
    const result = (await response.json()) as { warnings: string[] };
    const current = read();
    if (current.key === state.key) localStorage.removeItem(KEY);
    else
      write(
        current.items
          .map((item) => ({
            ...item,
            quantity: Math.max(
              0,
              item.quantity -
                (state.items.find((old) => old.product.id === item.product.id)
                  ?.quantity || 0),
            ),
          }))
          .filter((item) => item.quantity > 0),
      );
    if (result.warnings?.length)
      sessionStorage.setItem("cart-merge-notice", result.warnings.join(" "));
    cartChanged();
  })().finally(() => {
    merging = undefined;
  });
  return merging;
}
