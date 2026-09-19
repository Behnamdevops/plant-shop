import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { getArticleBySlug, type Article } from '../api/articles'
import { marked } from 'marked'
import DOMPurify from 'dompurify'

// Configure marked to be safe - no HTML rendering
marked.setOptions({
  breaks: true,
  gfm: true,
})

export default function ArticlePage() {
  const { slug } = useParams<{ slug: string }>()
  const [article, setArticle] = useState<Article | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!slug) return

    getArticleBySlug(slug)
      .then((data) => {
        setArticle(data)
        setError(null)
        
        // Update SEO metadata
        const seoTitle = data.seo_title || data.title
        const seoDesc = data.seo_description || data.excerpt
        
        document.title = seoTitle
        const descMeta = document.querySelector('meta[name="description"]')
        if (descMeta) {
          descMeta.setAttribute('content', seoDesc)
        } else {
          const meta = document.createElement('meta')
          meta.name = 'description'
          meta.content = seoDesc
          document.head.appendChild(meta)
        }
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : 'مقاله یافت نشد')
      })
      .finally(() => setLoading(false))
  }, [slug])

  const formatDate = (dateStr: string | null) => {
    if (!dateStr) return ''
    return new Date(dateStr).toLocaleDateString('fa-IR', {
      year: 'numeric',
      month: 'long',
      day: 'numeric'
    })
  }

  const renderMarkdown = (content: string) => {
    // Parse markdown to HTML, then sanitize to prevent XSS
    const html = marked.parse(content) as string
    // Sanitize: removes script tags, event handlers, javascript: links, etc.
    const cleanHtml = DOMPurify.sanitize(html, {
      ALLOWED_TAGS: ['p', 'br', 'strong', 'em', 'b', 'i', 'u', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
                     'ul', 'ol', 'li', 'a', 'blockquote', 'code', 'pre', 'hr', 'img', 'table',
                     'thead', 'tbody', 'tr', 'th', 'td'],
      ALLOWED_ATTR: ['href', 'src', 'alt', 'title', 'class'],
      ALLOW_DATA_ATTR: false,
    })
    return { __html: cleanHtml }
  }

  if (loading) {
    return <div className="loading">در حال بارگذاری...</div>
  }

  if (error || !article) {
    return (
      <div className="error-page">
        <h2>مقاله یافت نشد</h2>
        <p>{error || 'این مقاله ممکن است حذف شده یا منتشر نشده باشد.'}</p>
        <Link to="/articles" className="back-link">بازگشت به مقالات</Link>
      </div>
    )
  }

  return (
    <div className="article-page">
      <article className="article">
        {article.cover_image_url && (
          <div className="article-hero">
            <img 
              src={article.cover_image_url} 
              alt={article.title}
              onError={(e) => {
                (e.target as HTMLImageElement).src = '/placeholder-image.jpg'
              }}
            />
          </div>
        )}

        <header className="article-header">
          {article.category && (
            <Link 
              to={`/articles?category=${article.category.id}`} 
              className="article-category"
            >
              {article.category.name}
            </Link>
          )}
          <h1>{article.title}</h1>
          <div className="article-meta">
            <span className="article-date">
              {formatDate(article.published_at)}
            </span>
          </div>
        </header>

        <div 
          className="article-body markdown-content"
          dangerouslySetInnerHTML={renderMarkdown(article.content)}
        />
      </article>

      <div className="article-footer">
        <Link to="/articles" className="back-link">
          ← بازگشت به مقالات
        </Link>
      </div>
    </div>
  )
}