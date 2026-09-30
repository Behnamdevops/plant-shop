import { Link } from "react-router-dom";
import { useState } from "react";
import type { Product } from "../types/product";
import ProductImage from "./ProductImage";
import WishlistButton from "./WishlistButton";
import { formatToman } from "../lib/format";
import { addCartItem } from "../api/cart";
import { useAuth } from "../hooks/useAuth";
import { kindLabel } from "../catalog";
import Icon from "./Icon";
export default function ProductCard({ product: p }: { product: Product }) {
  const { loading } = useAuth();
  const expired =
    !!p.details?.expiry_date &&
    p.details.expiry_date < new Date().toISOString().slice(0, 10);
  const [busy, setBusy] = useState(false),
    [message, setMessage] = useState(""),
    [error, setError] = useState("");
  return (
    <article className="product-card">
      <div className="product-card__media">
        <Link to={`/products/${p.slug}`} className="product-card__image-link">
          <ProductImage
            src={p.image_url}
            alt={p.name}
            className="product-card__image"
            placeholderClassName="product-card__placeholder"
          />
        </Link>
        <WishlistButton product={p} />
        <span className={`product-stock ${p.stock > 0 ? "is-available" : ""}`}>
          {expired ? "پایان تاریخ مصرف" : p.stock > 0 ? "موجود" : "ناموجود"}
        </span>
      </div>
      <div className="product-card__body">
        <div className="product-card__care">
          {p.details?.brand || kindLabel(p.details?.kind)}
          {p.details?.weight_volume ? ` · ${p.details.weight_volume}` : ""}
        </div>
        <h3>
          <Link to={`/products/${p.slug}`}>{p.name}</Link>
        </h3>
        <div className="product-card__purchase">
          <p className="price">{formatToman(p.price)}</p>
          <button
            className="quick-add"
            type="button"
            aria-label={`افزودن ${p.name} به سبد خرید`}
            disabled={loading || busy || p.stock < 1 || expired}
            onClick={async () => {
              setBusy(true);
              setMessage("");
              setError("");
              try {
                await addCartItem(p.id, 1, p);
                setMessage("به سبد اضافه شد");
              } catch (err) {
                setError(
                  err instanceof Error ? err.message : "افزودن انجام نشد.",
                );
              } finally {
                setBusy(false);
              }
            }}
          >
            <Icon name="plus" />
            {busy && <span className="sr-only">در حال افزودن</span>}
          </button>
        </div>
        {message && (
          <p className="card-feedback" role="status">
            {message} <Link to="/cart">مشاهده</Link>
          </p>
        )}
        {error && (
          <p className="card-feedback is-error" role="alert">
            {error}
          </p>
        )}
      </div>
    </article>
  );
}
