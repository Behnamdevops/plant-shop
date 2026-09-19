import { useEffect, useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { getProducts } from '../api/products'
import { getCategories } from '../api/categories'
import type { Product, ProductSort } from '../types/product'
import type { Category } from '../types/category'
import { storeConfig } from '../config'
import { formatToman } from '../lib/format'
import { useDebouncedValue } from '../hooks/useDebouncedValue'
import ProductImage from '../components/ProductImage'
import SectionHeader from '../components/SectionHeader'
import Hero from '../components/Hero'

const VALID_SORTS: ProductSort[] = ['newest', 'price_asc', 'price_desc', 'name_asc']
const PAGE_SIZE = 20

const SORT_LABELS: Record<ProductSort, string> = {
  newest: 'جدیدترین',
  price_asc: 'قیمت: کم به زیاد',
  price_desc: 'قیمت: زیاد به کم',
  name_asc: 'نام: الف تا ی',
}

function isValidSort(value: string | null): value is ProductSort {
  return VALID_SORTS.includes(value as ProductSort)
}

function parsePositiveInt(value: string | null): number | undefined {
  if (!value) return undefined
  const n = Number(value)
  return Number.isFinite(n) && n > 0 ? Math.floor(n) : undefined
}

function stockLabel(stock: number): { text: string; className: string } {
  if (stock <= 0) return { text: 'ناموجود', className: 'badge-out-of-stock' }
  if (stock <= 5) return { text: 'تعداد محدود', className: 'badge-limited-stock' }
  return { text: 'موجود', className: 'badge-in-stock' }
}

export default function ShopPage() {
  const [searchParams, setSearchParams] = useSearchParams()

  const [queryInput, setQueryInput] = useState(searchParams.get('q') ?? '')
  const debouncedQuery = useDebouncedValue(queryInput, 400)

  const [minPriceInput, setMinPriceInput] = useState(searchParams.get('min_price') ?? '')
  const [maxPriceInput, setMaxPriceInput] = useState(searchParams.get('max_price') ?? '')
  const debouncedMinPrice = useDebouncedValue(minPriceInput, 400)
  const debouncedMaxPrice = useDebouncedValue(maxPriceInput, 400)

  const [categories, setCategories] = useState<Category[]>([])

  const [products, setProducts] = useState<Product[]>([])
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const sort = isValidSort(searchParams.get('sort')) ? (searchParams.get('sort') as ProductSort) : 'newest'
  const categoryParam = parsePositiveInt(searchParams.get('category'))
  const inStock = searchParams.get('in_stock') === 'true'
  const minPrice = parsePositiveInt(searchParams.get('min_price'))
  const maxPrice = parsePositiveInt(searchParams.get('max_price'))
  const page = Math.max(1, parsePositiveInt(searchParams.get('page')) ?? 1)
  const q = searchParams.get('q') ?? ''

  const updateParams = (patch: Record<string, string | undefined>, resetPage = true) => {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        for (const [key, value] of Object.entries(patch)) {
          if (value === undefined || value === '') {
            next.delete(key)
          } else {
            next.set(key, value)
          }
        }
        if (resetPage) {
          next.delete('page')
        }
        return next
      },
      { replace: true },
    )
  }

  useEffect(() => {
    if (debouncedQuery !== q) {
      updateParams({ q: debouncedQuery || undefined })
    }
  }, [debouncedQuery])

  useEffect(() => {
    const current = searchParams.get('min_price') ?? ''
    if (debouncedMinPrice !== current) {
      updateParams({ min_price: debouncedMinPrice || undefined })
    }
  }, [debouncedMinPrice])

  useEffect(() => {
    const current = searchParams.get('max_price') ?? ''
    if (debouncedMaxPrice !== current) {
      updateParams({ max_price: debouncedMaxPrice || undefined })
    }
  }, [debouncedMaxPrice])

  useEffect(() => {
    getCategories()
      .then(setCategories)
      .catch(() => {
        /* Category filter is a progressive enhancement */
      })
  }, [])

  const filtersKey = useMemo(
    () => JSON.stringify({ q, categoryParam, inStock, minPrice, maxPrice, sort, page }),
    [q, categoryParam, inStock, minPrice, maxPrice, sort, page],
  )

  useEffect(() => {
    let ignore = false

    async function load() {
      setLoading(true)
      try {
        const result = await getProducts({
          q: q || undefined,
          category: categoryParam,
          in_stock: inStock || undefined,
          min_price: minPrice,
          max_price: maxPrice,
          sort,
          page,
          page_size: PAGE_SIZE,
        })
        if (ignore) return
        setProducts(result.items)
        setTotal(result.total)
        setTotalPages(result.total_pages)
        setError('')
      } catch {
        if (!ignore) setError('مشکلی در بارگذاری محصولات پیش آمد')
      } finally {
        if (!ignore) setLoading(false)
      }
    }

    load()

    return () => {
      ignore = true
    }
  }, [filtersKey])

  const hasActiveFilters = Boolean(q || categoryParam || inStock || minPrice || maxPrice || sort !== 'newest')

  const clearFilters = () => {
    setQueryInput('')
    setMinPriceInput('')
    setMaxPriceInput('')
    setSearchParams(new URLSearchParams(), { replace: true })
  }

  const goToPage = (nextPage: number) => {
    if (nextPage < 1 || (totalPages > 0 && nextPage > totalPages)) return
    updateParams({ page: nextPage === 1 ? undefined : String(nextPage) }, false)
  }

  return (
    <div className="shop-page">
      <Hero
        title={`فروشگاه ${storeConfig.name}`}
        subtitle="گیاهان سالم و تازه، تا در خانه شما"
        primaryText="مشاهده دسته‌بندی‌ها"
        primaryLink="/shop"
        secondaryText="مشاهده مقالات"
        secondaryLink="/articles"
      />

      <div className="page-content">
        <SectionHeader
          title="فیلتر و جستجو"
          subtitle="محصولات خود را پیدا کنید"
          align="left"
        />

        <div className="catalog-filters">
          <input
            type="search"
            className="catalog-filters__search"
            placeholder="جستجوی محصول..."
            value={queryInput}
            onChange={(event) => setQueryInput(event.target.value)}
            aria-label="جستجوی محصول"
          />

          <select
            value={categoryParam ?? ''}
            onChange={(event) => updateParams({ category: event.target.value || undefined })}
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
              onChange={(event) => updateParams({ in_stock: event.target.checked ? 'true' : undefined })}
            />
            فقط موجود
          </label>

          <input
            type="number"
            min={0}
            className="catalog-filters__price"
            placeholder="حداقل قیمت"
            value={minPriceInput}
            onChange={(event) => setMinPriceInput(event.target.value)}
            aria-label="حداقل قیمت (ریال)"
          />

          <input
            type="number"
            min={0}
            className="catalog-filters__price"
            placeholder="حداکثر قیمت"
            value={maxPriceInput}
            onChange={(event) => setMaxPriceInput(event.target.value)}
            aria-label="حداکثر قیمت (ریال)"
          />

          <select value={sort} onChange={(event) => updateParams({ sort: event.target.value })} aria-label="مرتب‌سازی">
            {VALID_SORTS.map((mode) => (
              <option key={mode} value={mode}>
                {SORT_LABELS[mode]}
              </option>
            ))}
          </select>

          {hasActiveFilters && (
            <button type="button" className="btn btn-secondary btn-sm" onClick={clearFilters}>
              حذف فیلترها
            </button>
          )}
        </div>

        {loading && (
          <p className="state-message">در حال بارگذاری محصولات...</p>
        )}

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
              {products.map((product) => {
                const stock = stockLabel(product.stock)
                return (
                  <article key={product.id} className="card product-card">
                    <div className="product-card__media">
                      <ProductImage
                        src={product.image_url}
                        alt={product.name}
                        placeholderClassName="product-card__media-placeholder"
                      />
                    </div>

                    <div className="product-card__body">
                      <h2>
                        <Link to={`/products/${product.slug}`}>{product.name}</Link>
                      </h2>

                      <p className="product-card__desc">{product.description}</p>

                      <div className="product-card__footer">
                        <span className="price">{formatToman(product.price)}</span>
                        <span className={`badge ${stock.className}`}>{stock.text}</span>
                      </div>

                      <Link to={`/products/${product.slug}`} className="btn btn-secondary btn-block">
                        مشاهده جزئیات
                      </Link>
                    </div>
                  </article>
                )
              })}
            </div>

            <nav className="pagination" aria-label="صفحه‌بندی محصولات">
              <button type="button" className="btn btn-secondary btn-sm" onClick={() => goToPage(page - 1)} disabled={page <= 1}>
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
  )
}
