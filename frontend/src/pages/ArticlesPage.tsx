import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { getArticleCategories, getArticles, type Article, type ArticleCategory } from '../api/articles'

export default function ArticlesPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [articles, setArticles] = useState<Article[]>([])
  const [categories, setCategories] = useState<ArticleCategory[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [totalPages, setTotalPages] = useState(0)

  const q = searchParams.get('q') || ''
  const category = searchParams.get('category')
  const page = parseInt(searchParams.get('page') || '1', 10)

  useEffect(() => {
    Promise.all([
      getArticleCategories(),
      getArticles({ q: q || undefined, category: category ? parseInt(category, 10) : undefined, page, page_size: 12 })
    ])
      .then(([cats, articlesData]) => {
        setCategories(cats)
        setArticles(articlesData.items)
        setTotalPages(articlesData.total_pages)
        setError(null)
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : 'خطا در دریافت مقالات')
      })
      .finally(() => setLoading(false))
  }, [q, category, page])

  const handleSearch = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const formData = new FormData(e.currentTarget)
    const newQ = formData.get('q') as string
    const params = new URLSearchParams(searchParams)
    if (newQ) {
      params.set('q', newQ)
    } else {
      params.delete('q')
    }
    params.delete('page')
    setSearchParams(params)
  }

  const handleCategoryClick = (catId: number | null) => {
    const params = new URLSearchParams(searchParams)
    if (catId) {
      params.set('category', String(catId))
    } else {
      params.delete('category')
    }
    params.delete('page')
    setSearchParams(params)
  }

  const handlePageChange = (newPage: number) => {
    const params = new URLSearchParams(searchParams)
    params.set('page', String(newPage))
    setSearchParams(params)
  }

  const formatDate = (dateStr: string | null) => {
    if (!dateStr) return ''
    return new Date(dateStr).toLocaleDateString('fa-IR', {
      year: 'numeric',
      month: 'long',
      day: 'numeric'
    })
  }

  if (loading) {
    return <div className="loading">در حال بارگذاری...</div>
  }

  return (
    <div className="articles-page">
      <div className="articles-header">
        <h1>مقالات و آموزش‌ها</h1>
        
        <form onSubmit={handleSearch} className="search-form">
          <input
            type="text"
            name="q"
            placeholder="جستجو در مقالات..."
            defaultValue={q}
            className="search-input"
          />
          <button type="submit" className="search-btn">جستجو</button>
        </form>
      </div>

      <div className="articles-filters">
        <button
          className={`filter-btn ${!category ? 'active' : ''}`}
          onClick={() => handleCategoryClick(null)}
        >
          همه
        </button>
        {categories.map((cat) => (
          <button
            key={cat.id}
            className={`filter-btn ${category === String(cat.id) ? 'active' : ''}`}
            onClick={() => handleCategoryClick(cat.id)}
          >
            {cat.name}
          </button>
        ))}
      </div>

      {error && <div className="error-message">{error}</div>}

      {articles.length === 0 ? (
        <div className="empty-state">
          <p>مقاله‌ای یافت نشد.</p>
        </div>
      ) : (
        <>
          <div className="articles-grid">
            {articles.map((article) => (
              <Link to={`/articles/${article.slug}`} key={article.id} className="article-card">
                {article.cover_image_url && (
                  <div className="article-cover">
                    <img src={article.cover_image_url} alt={article.title} />
                  </div>
                )}
                <div className="article-content">
                  {article.category && (
                    <span className="article-category">{article.category.name}</span>
                  )}
                  <h3>{article.title}</h3>
                  <p className="article-excerpt">{article.excerpt}</p>
                  <span className="article-date">
                    {formatDate(article.published_at)}
                  </span>
                </div>
              </Link>
            ))}
          </div>

          {totalPages > 1 && (
            <div className="pagination">
              <button
                disabled={page <= 1}
                onClick={() => handlePageChange(page - 1)}
                className="pagination-btn"
              >
                قبلی
              </button>
              <span className="page-info">
                صفحه {page} از {totalPages}
              </span>
              <button
                disabled={page >= totalPages}
                onClick={() => handlePageChange(page + 1)}
                className="pagination-btn"
              >
                بعدی
              </button>
            </div>
          )}
        </>
      )}
    </div>
  )
}