import { usePublicData } from "../context/PublicDataContext";
import ProductCard from "../components/ProductCard";
import { useEffect, useMemo, useState, useCallback } from "react";
import { useSearchParams } from "react-router-dom";
import { getProducts } from "../api/products";
import { getCategories } from "../api/categories";
import type { Product, ProductSort } from "../types/product";
import type { Category } from "../types/category";
import { storeConfig } from "../config";
import { useDebouncedValue } from "../hooks/useDebouncedValue";
import SectionHeader from "../components/SectionHeader";
import Icon from "../components/Icon";
import type { Category as CatalogCategory } from "../types/category";
import { productKinds } from "../catalog";
import SEO from "../components/SEO";

const VALID_SORTS: ProductSort[] = [
  "newest",
  "price_asc",
  "price_desc",
  "name_asc",
  "bestselling",
];
const PAGE_SIZE = 20;

const SORT_LABELS: Record<ProductSort, string> = {
  newest: "جدیدترین",
  bestselling: "پرفروش‌ترین",
  price_asc: "قیمت: کم به زیاد",
  price_desc: "قیمت: زیاد به کم",
  name_asc: "نام: الف تا ی",
};

function isValidSort(value: string | null): value is ProductSort {
  return VALID_SORTS.includes(value as ProductSort);
}

function parsePositiveInt(value: string | null): number | undefined {
  if (!value) return undefined;
  const n = Number(value);
  return Number.isFinite(n) && n > 0 ? Math.floor(n) : undefined;
}

// A pending edit belongs to the URL it started from. Navigation and clearing
// filters must not be overwritten by an old debounce callback.
function useCatalogDraft(current: string, commit: (value: string) => void) {
  const [draft, setDraft] = useState({ base: current, value: current });
  const debounced = useDebouncedValue(draft, 400);
  useEffect(() => {
    if (debounced.base === current && debounced.value !== current)
      commit(debounced.value);
  }, [debounced, current, commit]);
  return [
    draft.base === current ? draft.value : current,
    (value: string) => setDraft({ base: current, value }),
  ] as const;
}

export default function ShopPage({
  category,
}: { category?: CatalogCategory } = {}) {
  const bootstrap = usePublicData();
  const [searchParams, setSearchParams] = useSearchParams();

  useEffect(() => {
    document.title = `فروشگاه ${storeConfig.name} - محصولات مراقبت از گیاه`;
    const descMeta = document.querySelector('meta[name="description"]');
    if (descMeta) {
      descMeta.setAttribute(
        "content",
        `خرید کود، خاک و محصولات مراقبت از گیاه از ${storeConfig.name} - آموزش نگهداری گیاهان`,
      );
    }
  }, []);

  const [categories, setCategories] = useState<Category[]>(
    bootstrap?.categories || [],
  );

  const [products, setProducts] = useState<Product[]>(
    bootstrap?.products?.items || [],
  );
  const [total, setTotal] = useState(bootstrap?.products?.total || 0);
  const [totalPages, setTotalPages] = useState(
    bootstrap?.products?.total_pages || bootstrap?.articles?.total_pages || 0,
  );
  const [loading, setLoading] = useState(!bootstrap);
  const [error, setError] = useState("");

  const sort = isValidSort(searchParams.get("sort"))
    ? (searchParams.get("sort") as ProductSort)
    : "newest";
  const categoryParam =
    category?.id || parsePositiveInt(searchParams.get("category"));
  const inStock = searchParams.get("in_stock") === "true";
  const minPrice = parsePositiveInt(searchParams.get("min_price"));
  const maxPrice = parsePositiveInt(searchParams.get("max_price"));
  const page = Math.max(1, parsePositiveInt(searchParams.get("page")) ?? 1);
  const kind = searchParams.get("kind") || undefined;
  const brand = searchParams.get("brand") || undefined;
  const formulation = searchParams.get("formulation") || undefined;
  const q = searchParams.get("q") ?? "";

  const updateParams = useCallback(
    (patch: Record<string, string | undefined>, resetPage = true) => {
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          for (const [key, value] of Object.entries(patch)) {
            if (value === undefined || value === "") {
              next.delete(key);
            } else {
              next.set(key, value);
            }
          }
          if (resetPage) {
            next.delete("page");
          }
          return next;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  const [queryInput, setQueryInput] = useCatalogDraft(q, (value) =>
    updateParams({ q: value || undefined }),
  );
  const [minPriceInput, setMinPriceInput] = useCatalogDraft(
    searchParams.get("min_price")
      ? String(Number(searchParams.get("min_price")) / 10)
      : "",
    (value) =>
      updateParams({
        min_price: value ? String(Math.round(Number(value) * 10)) : undefined,
      }),
  );
  const [maxPriceInput, setMaxPriceInput] = useCatalogDraft(
    searchParams.get("max_price")
      ? String(Number(searchParams.get("max_price")) / 10)
      : "",
    (value) =>
      updateParams({
        max_price: value ? String(Math.round(Number(value) * 10)) : undefined,
      }),
  );

  useEffect(() => {
    getCategories()
      .then(setCategories)
      .catch(() => {
        /* Category filter is a progressive enhancement */
      });
  }, []);

  const filtersKey = useMemo(
    () =>
      JSON.stringify({
        q,
        categoryParam,
        inStock,
        minPrice,
        maxPrice,
        sort,
        page,
        kind,
        brand,
        formulation,
      }),
    [
      q,
      categoryParam,
      inStock,
      minPrice,
      maxPrice,
      sort,
      page,
      kind,
      brand,
      formulation,
    ],
  );

  useEffect(() => {
    let ignore = false;

    async function load() {
      setLoading(true);
      try {
        const result = await getProducts({
          q: q || undefined,
          kind,
          brand,
          formulation,
          category: categoryParam,
          in_stock: inStock || undefined,
          min_price: minPrice,
          max_price: maxPrice,
          sort,
          page,
          page_size: PAGE_SIZE,
        });
        if (ignore) return;
        setProducts(result.items);
        setTotal(result.total);
        setTotalPages(result.total_pages);
        setError("");
      } catch {
        if (!ignore) setError("مشکلی در بارگذاری محصولات پیش آمد");
      } finally {
        if (!ignore) setLoading(false);
      }
    }

    load();

    return () => {
      ignore = true;
    };
  }, [
    filtersKey,
    q,
    categoryParam,
    inStock,
    minPrice,
    maxPrice,
    sort,
    page,
    kind,
    brand,
    formulation,
  ]);

  const hasActiveFilters = Boolean(
    q ||
      categoryParam ||
      inStock ||
      minPrice ||
      maxPrice ||
      kind ||
      brand ||
      formulation ||
      sort !== "newest",
  );

  const clearFilters = () => {
    setQueryInput("");
    setMinPriceInput("");
    setMaxPriceInput("");
    setSearchParams(new URLSearchParams(), { replace: true });
  };

  const goToPage = (nextPage: number) => {
    if (nextPage < 1 || (totalPages > 0 && nextPage > totalPages)) return;
    updateParams(
      { page: nextPage === 1 ? undefined : String(nextPage) },
      false,
    );
  };

  return (
    <div className="shop-page">
      <SEO
        title={
          category ? `خرید ${category.name}` : "خرید محصولات مراقبت از گیاه"
        }
        description={`خرید کود، خاک و محصولات مراقبت از گیاه از ${storeConfig.name}`}
        canonical={category ? `/category/${category.slug}` : "/shop"}
      />
      <div className="catalog-intro">
        <span className="eyebrow">کلکسیون سبز ما</span>
        <h1>{category?.name || "همه چیز برای مراقبت بهتر از گیاه"}</h1>
        <p>کود، خاک، ابزار و آموزش را بر اساس مشخصات و روش مصرف انتخاب کنید.</p>
        <div className="category-pills">
          <button
            type="button"
            className={!categoryParam ? "is-selected" : ""}
            onClick={() => {
              if (category) window.location.assign("/shop");
              else updateParams({ category: undefined });
            }}
          >
            همهٔ محصولات
          </button>
          {categories.map((c) => (
            <button
              key={c.id}
              type="button"
              className={categoryParam === c.id ? "is-selected" : ""}
              onClick={() => {
                if (category) window.location.assign(`/category/${c.slug}`);
                else updateParams({ category: String(c.id) });
              }}
            >
              {c.name}
            </button>
          ))}
        </div>
      </div>

      <div className="page-content">
        <SectionHeader
          title="انتخاب را دقیق‌تر کنید"
          subtitle="محصولات خود را پیدا کنید"
          align="left"
        />

        <div className="catalog-filters">
          <span className="filter-symbol">
            <Icon name="search" size={19} />
          </span>
          <input
            type="search"
            className="catalog-filters__search"
            placeholder="جستجوی محصول..."
            value={queryInput}
            onChange={(event) => setQueryInput(event.target.value)}
            aria-label="جستجوی محصول"
          />

          <select
            value={categoryParam ?? ""}
            onChange={(event) => {
              if (category) {
                const selected = categories.find(
                  (item) => String(item.id) === event.target.value,
                );
                window.location.assign(
                  selected ? `/category/${selected.slug}` : "/shop",
                );
              } else {
                updateParams({ category: event.target.value || undefined });
              }
            }}
            aria-label="دسته‌بندی"
          >
            <option value="">همه دسته‌بندی‌ها</option>
            {categories.map((category) => (
              <option key={category.id} value={category.id}>
                {category.name}
              </option>
            ))}
          </select>

          <label className="catalog-filters__checkbox">
            <input
              type="checkbox"
              checked={inStock}
              onChange={(event) =>
                updateParams({
                  in_stock: event.target.checked ? "true" : undefined,
                })
              }
            />
            فقط موجود
          </label>

          <input
            type="number"
            min={0}
            className="catalog-filters__price"
            placeholder="حداقل قیمت · تومان"
            value={minPriceInput}
            onChange={(event) => setMinPriceInput(event.target.value)}
            aria-label="حداقل قیمت (تومان)"
          />

          <input
            type="number"
            min={0}
            className="catalog-filters__price"
            placeholder="حداکثر قیمت · تومان"
            value={maxPriceInput}
            onChange={(event) => setMaxPriceInput(event.target.value)}
            aria-label="حداکثر قیمت (تومان)"
          />

          <select
            value={sort}
            onChange={(event) => updateParams({ sort: event.target.value })}
            aria-label="مرتب‌سازی"
          >
            {VALID_SORTS.map((mode) => (
              <option key={mode} value={mode}>
                {SORT_LABELS[mode]}
              </option>
            ))}
          </select>

          {hasActiveFilters && (
            <button
              type="button"
              className="btn btn-secondary btn-sm"
              onClick={clearFilters}
            >
              حذف فیلترها
            </button>
          )}
        </div>

        <div className="care-filters">
          <label>
            نوع محصول
            <select
              value={kind || ""}
              onChange={(e) => updateParams({ kind: e.target.value })}
            >
              <option value="">همهٔ انواع</option>
              {productKinds.map((k) => (
                <option key={k.value} value={k.value}>
                  {k.label}
                </option>
              ))}
            </select>
          </label>
          <label>
            برند
            <input
              value={brand || ""}
              onChange={(e) => updateParams({ brand: e.target.value })}
              placeholder="نام دقیق برند"
              maxLength={160}
            />
          </label>
          <label>
            نوع مصرف
            <input
              value={formulation || ""}
              onChange={(e) => updateParams({ formulation: e.target.value })}
              placeholder="مثلاً مایع یا پودری"
              maxLength={120}
            />
          </label>
        </div>
        {loading && <p className="state-message">در حال بارگذاری محصولات...</p>}

        {!loading && error && (
          <p className="alert alert-error" role="alert">
            {error}
          </p>
        )}

        {!loading && !error && products.length === 0 && (
          <p className="empty-state">محصولی با این فیلترها یافت نشد.</p>
        )}

        {!loading && !error && products.length > 0 && (
          <>
            <div className="product-grid">
              {products.map((product) => (
                <ProductCard key={product.id} product={product} />
              ))}
            </div>

            <nav className="pagination" aria-label="صفحه‌بندی محصولات">
              <button
                type="button"
                className="btn btn-secondary btn-sm"
                onClick={() => goToPage(page - 1)}
                disabled={page <= 1}
              >
                قبلی
              </button>
              <span className="pagination__status">
                صفحه {page} از {Math.max(totalPages, 1)} ({total} محصول)
              </span>
              <button
                type="button"
                className="btn btn-secondary btn-sm"
                onClick={() => goToPage(page + 1)}
                disabled={totalPages === 0 || page >= totalPages}
              >
                بعدی
              </button>
            </nav>
          </>
        )}
      </div>
    </div>
  );
}
