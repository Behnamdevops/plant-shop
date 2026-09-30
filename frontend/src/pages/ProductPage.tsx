import { usePublicData } from "../context/PublicDataContext";
import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { getProduct, getRelatedProducts } from "../api/products";
import { addCartItem } from "../api/cart";
import type { Product } from "../types/product";
import { useAuth } from "../hooks/useAuth";
import { formatToman } from "../lib/format";
import { storeConfig } from "../config";
import ProductImage from "../components/ProductImage";
import ProductCard from "../components/ProductCard";
import WishlistButton from "../components/WishlistButton";
import ProductReviews from "../components/ProductReviews";
import { kindLabel } from "../catalog";
import SEO from "../components/SEO";
function ProductDetails({ slug }: { slug: string }) {
  const bootstrap = usePublicData();
  const { loading: authLoading } = useAuth();
  const [product, setProduct] = useState<Product | null>(
      bootstrap?.product || null,
    ),
    [loading, setLoading] = useState(!bootstrap),
    [error, setError] = useState(""),
    [quantity, setQuantity] = useState(1),
    [selected, setSelected] = useState<string | null>(null),
    [related, setRelated] = useState<Product[]>(
      bootstrap?.relatedProducts || [],
    ),
    [busy, setBusy] = useState(false),
    [message, setMessage] = useState("");
  useEffect(() => {
    let active = true;
    getProduct(slug)
      .then((value) => {
        if (active) setProduct(value);
      })
      .catch(() => {
        if (active) setError("محصول یافت نشد یا دریافت اطلاعات انجام نشد.");
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [slug]);
  useEffect(() => {
    if (!product?.slug) return;
    let active = true;
    getRelatedProducts(product.slug)
      .then((value) => {
        if (active) setRelated(value);
      })
      .catch(() => {});
    return () => {
      active = false;
    };
  }, [product?.id, product?.slug]);
  if (loading)
    return (
      <main>
        <p>در حال بارگذاری محصول...</p>
      </main>
    );
  if (!product)
    return (
      <main>
        <SEO title="محصول یافت نشد" noindex />
        <h1>محصول یافت نشد</h1>
        <p>{error}</p>
        <Link to="/shop">بازگشت به فروشگاه</Link>
      </main>
    );
  const details = product.details || {};
  const expired =
    !!details.expiry_date &&
    details.expiry_date < new Date().toISOString().slice(0, 10);
  const images = [
    ...new Set(
      [product.image_url, ...(product.image_urls || [])].filter(
        (x): x is string => !!x,
      ),
    ),
  ];
  const specs = [
    ["نوع محصول", kindLabel(details.kind)],
    ["برند", details.brand],
    ["وزن / حجم", details.weight_volume],
    ["نوع مصرف", details.formulation],
    ["مناسب برای", details.suitable_for],
    [
      "تعداد در بسته",
      details.pack_count ? String(details.pack_count) : undefined,
    ],
    ["کشور سازنده", details.country],
    ["تاریخ انقضا", details.expiry_date],
    ["اقلام داخل بسته", details.included],
  ];
  const structured = {
    "@context": "https://schema.org",
    "@type": "Product",
    name: product.name,
    description: product.description,
    image: images,
    sku: String(product.id),
    ...(details.brand
      ? { brand: { "@type": "Brand", name: details.brand } }
      : {}),
    additionalProperty: specs
      .filter(([, v]) => v)
      .map(([name, value]) => ({ "@type": "PropertyValue", name, value })),
    offers: {
      "@type": "Offer",
      url:
        (bootstrap?.origin || storeConfig.publicOrigin) +
        "/products/" +
        product.slug,
      price: product.price,
      priceCurrency: "IRR",
      availability:
        product.stock > 0 && !expired
          ? "https://schema.org/InStock"
          : "https://schema.org/OutOfStock",
    },
  };
  return (
    <main>
      <SEO
        title={product.name}
        description={product.description}
        canonical={`/products/${product.slug}`}
        ogImage={product.image_url || undefined}
      />
      <script type="application/ld+json">
        {JSON.stringify(structured).replace(/</g, "\\u003c")}
      </script>
      <Link className="back-link" to="/shop">
        بازگشت به فروشگاه
      </Link>
      <div className="product-detail">
        <div className="product-detail__media">
          <ProductImage
            priority
            src={selected || product.image_url}
            alt={product.name}
            placeholderClassName="product-detail__media-placeholder"
          />
          {images.length > 1 && (
            <div className="gallery-thumbs">
              {images.map((src) => (
                <button
                  key={src}
                  type="button"
                  aria-label="نمایش تصویر محصول"
                  aria-pressed={(selected || product.image_url) === src}
                  onClick={() => setSelected(src)}
                >
                  <ProductImage src={src} alt={product.name} />
                </button>
              ))}
            </div>
          )}
        </div>
        <div className="product-detail__info">
          <h1>{product.name}</h1>
          <p className="price">{formatToman(product.price)}</p>
          <p>
            {expired
              ? "تاریخ مصرف این محصول گذشته است"
              : product.stock > 0
                ? "موجود"
                : "ناموجود"}
          </p>
          <p className="product-detail__description">{product.description}</p>
          <dl className="care-specs">
            {specs
              .filter(([, v]) => v)
              .map(([label, value]) => (
                <div key={label}>
                  <dt>{label}</dt>
                  <dd>{value}</dd>
                </div>
              ))}
          </dl>
          {details.article_slug && (
            <p>
              <Link to={`/blog/${encodeURIComponent(details.article_slug)}`}>
                راهنمای انتخاب و مصرف این محصول
              </Link>
            </p>
          )}
          {(
            [
              ["composition", "ترکیبات"],
              ["benefits", "ویژگی‌ها و مزایا"],
              ["usage", "روش مصرف"],
              ["warnings", "هشدار مصرف"],
              ["delivery_info", "روش ارائهٔ آموزش و مشاوره"],
            ] as const
          )
            .filter(([key]) => details[key])
            .map(([key, label]) => (
              <section
                className={`product-text product-text--${key}`}
                key={key}
              >
                <h2>{label}</h2>
                <p>{details[key]}</p>
              </section>
            ))}
          <p>
            <Link to="/shipping">روش و هزینه ارسال</Link> ·{" "}
            <Link to="/returns">رسیدگی به آسیب هنگام حمل</Link>
          </p>
          <label>
            تعداد{" "}
            <input
              type="number"
              aria-label="تعداد محصول"
              min={1}
              max={product.stock}
              value={quantity}
              onChange={(e) => setQuantity(Math.max(1, Number(e.target.value)))}
            />
          </label>
          <button
            className="btn btn-primary"
            disabled={busy || authLoading || product.stock < 1 || expired}
            onClick={async () => {
              setBusy(true);
              setMessage("");
              setError("");
              try {
                await addCartItem(product.id, quantity, product);
                setMessage("به سبد خرید اضافه شد");
              } catch (err) {
                setError(
                  err instanceof Error ? err.message : "افزودن انجام نشد.",
                );
              } finally {
                setBusy(false);
              }
            }}
          >
            {busy ? "در حال افزودن..." : "افزودن به سبد خرید"}
          </button>{" "}
          <WishlistButton product={product} />
          {message && (
            <p role="status">
              {message} <Link to="/cart">مشاهده سبد خرید</Link>
            </p>
          )}
          {error && <p role="alert">{error}</p>}
        </div>
      </div>
      {related.length > 0 && (
        <section>
          <h2>محصولات مرتبط و مکمل</h2>
          <div className="product-grid">
            {related.map((p) => (
              <ProductCard key={p.id} product={p} />
            ))}
          </div>
        </section>
      )}
      <ProductReviews productID={product.id} />
    </main>
  );
}
export default function ProductPage() {
  const { slug } = useParams();
  return slug ? (
    <ProductDetails key={slug} slug={slug} />
  ) : (
    <p>محصول یافت نشد</p>
  );
}
