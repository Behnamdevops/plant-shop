import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getProducts } from '../api/products'
import { getCategories } from '../api/categories'
import { getArticles } from '../api/articles'
import type { Product } from '../types/product'
import type { Article } from '../api/articles'
import type { Category } from '../types/category'
import { storeConfig } from '../config'
import { formatToman } from '../lib/format'
import Hero from '../components/Hero'
import SectionHeader from '../components/SectionHeader'
import ArticleCard from '../components/ArticleCard'
import StoreBenefitCard from '../components/StoreBenefitCard'

export default function HomePage() {
  const [categories, setCategories] = useState<Category[]>([])
  const [products, setProducts] = useState<Product[]>([])
  const [articles, setArticles] = useState<Article[]>([])
  
  const [loadingProducts, setLoadingProducts] = useState(true)
  const [loadingArticles, setLoadingArticles] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    document.title = storeConfig.name + ' | گیاهات سالم برای زندگی سالم‌تر'
    const descMeta = document.querySelector('meta[name="description"]')
    if (descMeta) {
      descMeta.setAttribute(
        'content',
        'فروشگاه آنلاین ' + storeConfig.name + ' - محصولات با کیفیت، آموزش‌های جامع و مشاوره گیاهان'
      )
    }
  }, [])

  useEffect(() => {
    getCategories()
      .then(setCategories)
      .catch(() => {
        /* Categories are optional */
      })
  }, [])

  useEffect(() => {
    let ignore = false

    async function loadProducts() {
      setLoadingProducts(true)
      try {
        const result = await getProducts({ sort: 'newest', page_size: 6 })
        if (!ignore) {
          setProducts(result.items)
        }
      } catch {
        if (!ignore) setError('مشکلی در بارگذاری محصولات پیش آمد')
      } finally {
        if (!ignore) setLoadingProducts(false)
      }
    }

    loadProducts()

    return () => {
      ignore = true
    }
  }, [])

  useEffect(() => {
    let ignore = false

    async function loadArticles() {
      setLoadingArticles(true)
      try {
        const result = await getArticles({ page_size: 4 })
        if (!ignore) {
          setArticles(result.items)
        }
      } catch {
        if (!ignore) {
          /* Articles failure is non-critical */
        }
      } finally {
        if (!ignore) setLoadingArticles(false)
      }
    }

    loadArticles()

    return () => {
      ignore = true
    }
  }, [])

  const featuredCategories = categories.slice(0, 4)

  return (
    <div className="home-page">
      {/* Hero Section */}
      <Hero
        title="گیاهان سالم، زندگی سالم‌تر"
        subtitle="گیاهات اطراف خانه، محتوای آموزشی و محصولات انتخاب شده برای زندگی بهتر شما"
        primaryText="مشاهده محصولات"
        primaryLink="/shop"
        secondaryText="مشاهده مقالات"
        secondaryLink="/articles"
      />

      {/* Categories Section */}
      <div className="marketing-section">
        <div className="section-container">
          <SectionHeader
            title="دسته‌بندی‌های محبوب"
            subtitle="محصولات ما در دسته‌بندی‌های مختلف"
            align="center"
            linkTo="/shop"
            linkText="مشاهده همه"
          />
          
          {loadingProducts && !products.length ? (
            <p className="state-message">در حال بارگذاری...</p>
          ) : (
            <div className="category-grid">
              {featuredCategories.length === 0 ? (
                <p className="empty-state">دسته‌بندی‌ای یافت نشد</p>
              ) : (
                featuredCategories.map((category) => (
                  <Link
                    key={category.id}
                    to={`/shop?category=${category.id}`}
                    className="category-card"
                  >
                    <div className="category-card__icon">🌱</div>
                    <h3 className="category-card__title">{category.name}</h3>
                    <p className="category-card__count">
                      {category.slug === 'indoor-plants' ? 'گیاهات داخل خانه' :
                       category.slug === 'outdoor-plants' ? 'گیاهات اطراف خانه' :
                       category.slug === 'flowers' ? 'گل‌ها' :
                       'گیاهات'}
                    </p>
                  </Link>
                ))
              )}
            </div>
          )}
        </div>
      </div>

      {/* Featured Products Section */}
      <div className="marketing-section">
        <div className="section-container">
          <SectionHeader
            title="محصولات پیشنهادی"
            subtitle="جدیدترین و محبوب‌ترین محصولات ما"
            align="center"
            linkTo="/shop"
            linkText="مشاهده همه محصولات"
          />

          {loadingProducts && !products.length ? (
            <p className="state-message">در حال بارگذاری محصولات...</p>
          ) : (
            <div className="product-grid">
              {products.map((product) => (
                <article key={product.id} className="card product-card">
                  <div className="product-card__media">
                    <img
                      src={product.image_url || ''}
                      alt={product.name}
                      className="product-card__media-img"
                      onError={(e) => {
                        (e.target as HTMLImageElement).style.display = 'none'
                      }}
                    />
                    <span className="product-card__media-placeholder" aria-hidden="true">
                      🌱
                    </span>
                  </div>

                  <div className="product-card__body">
                    <h2>
                      <Link to={`/products/${product.slug}`}>{product.name}</Link>
                    </h2>

                    <p className="product-card__desc">{product.description}</p>

                    <div className="product-card__footer">
                      <span className="price">{formatToman(product.price)}</span>
                      {product.stock <= 0 ? (
                        <span className="badge badge-out-of-stock">ناموجود</span>
                      ) : product.stock <= 5 ? (
                        <span className="badge badge-limited-stock">تعداد محدود</span>
                      ) : (
                        <span className="badge badge-in-stock">موجود</span>
                      )}
                    </div>

                    <Link to={`/products/${product.slug}`} className="btn btn-secondary btn-block">
                      مشاهده جزئیات
                    </Link>
                  </div>
                </article>
              ))}
            </div>
          )}

          {!loadingProducts && products.length === 0 && !error && (
            <p className="empty-state">محصولی یافت نشد</p>
          )}
        </div>
      </div>

      {/* Educational Content Section */}
      <div className="marketing-section">
        <div className="section-container">
          <div className="content-section content-section--centered">
            <SectionHeader
              title="آموزش و آگاهی"
              subtitle="یاد بگیرید گیاهان خود را بهتر مراقبت کنید"
              align="center"
              linkTo="/articles"
              linkText="مشاهده تمام مقالات"
            />
            <div className="content-promo">
              <p>
                دانش نگهداری از گیاهان کلید موفقیت شماست. ما مجموعه‌ای از مقالات آموزشی
                برای همه سطوح داریم - از مبتدی تا حرفه‌ای.
              </p>
              <Link to="/articles" className="btn btn-primary">
                شروع به یادگیری
              </Link>
            </div>
          </div>
        </div>
      </div>

      {/* Latest Articles Section */}
      <div className="marketing-section">
        <div className="section-container">
          <SectionHeader
            title="آخرین مقالات"
            subtitle="مطالب جدید از دنیای گیاهان"
            align="center"
            linkTo="/articles"
            linkText="مشاهده همه مقالات"
          />

          {loadingArticles && !articles.length ? (
            <p className="state-message">در حال بارگذاری مقالات...</p>
          ) : (
            <div className="articles-grid">
              {articles.map((article) => (
                <ArticleCard key={article.id} article={article} showImage={false} />
              ))}
            </div>
          )}

          {!loadingArticles && articles.length === 0 && (
            <p className="empty-state">مقاله‌ای یافت نشد</p>
          )}
        </div>
      </div>

      {/* Store Benefits Section */}
      <div className="marketing-section">
        <div className="section-container">
          <SectionHeader
            title="چرا از ما انتخاب کنید؟"
            align="center"
          />
          <div className="benefits-grid">
            <StoreBenefitCard
              title="ارسال مطمئن"
              description="بسته‌بندی حرفه‌ای و ارسال ایمن برای سلامت گیاهان"
            />
            <StoreBenefitCard
              title="گیاهان سالم"
              description="فقط گیاهات با کیفیت و سالم ارائه می‌شود"
            />
            <StoreBenefitCard
              title="راهنمای نگهداری"
              description="آموزش‌های جامع برای مراقبت از گیاهان"
            />
            <StoreBenefitCard
              title="خرید امن"
              description="پرداخت آنلاین امن و راه‌های پشتیبانی متعدد"
            />
          </div>
        </div>
      </div>

      {/* Short FAQ Preview */}
      <div className="marketing-section">
        <div className="section-container">
          <SectionHeader
            title="سوالات معمول"
            align="center"
            linkTo="/faq"
            linkText="مشاهده تمام سوالات"
          />
          <div className="faq-preview">
            <div className="faq-item">
              <strong>چگونه سفارش دهم؟</strong>
              <p>محصول مورد نظر را انتخاب کرده و دکمه «افزودن به سبد خرید» را بزنید.</p>
            </div>
            <div className="faq-item">
              <strong>ارسال چقدر طول می‌کشد؟</strong>
              <p>متوسط زمان ارسال ۱ تا ۳ روز کاری است.</p>
            </div>
            <div className="faq-item">
              <strong>گیاه بیمار شد، چه کنم؟</strong>
              <p>با پشتیبانی تماس بگیرید تا بهترین راهنمایی را دریافت کنید.</p>
            </div>
            <div className="faq-item">
              <strong>بازگرداندن محصول؟</strong>
              <p>در صورت آسیب در حمل، با ما تماس بگیرید.</p>
            </div>
          </div>
        </div>
      </div>

      {/* Final CTA */}
      <div className="marketing-section">
        <div className="section-container">
          <div className="final-cta">
            <h2>آماده خرید هستید؟</h2>
            <p>گیاهان سالم، زندگی سالم‌تری را آغاز کنید</p>
            <div className="final-cta__buttons">
              <Link to="/shop" className="btn btn-primary">
                شروع خرید
              </Link>
              <Link to="/articles" className="btn btn-secondary">
                آموزش‌ها
              </Link>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
