import { Link } from 'react-router-dom'
import type { Article } from '../api/articles'
import { formatDateFa } from '../lib/format'

type ArticleCardProps = {
  article: Article
  showCategory?: boolean
  showImage?: boolean
}

export default function ArticleCard({
  article,
  showCategory = true,
  showImage = true
}: ArticleCardProps) {
  if (!article.published_at) return null

  return (
    <article className="article-card">
      {showImage && article.cover_image_url && (
        <div className="article-card__media">
          <img src={article.cover_image_url} alt={article.title} loading="lazy" />
        </div>
      )}
      <div className="article-card__content">
        {showCategory && article.category && (
          <span className="article-card__category">{article.category.name}</span>
        )}
        <h3 className="article-card__title">
          <Link to={`/articles/${article.slug}`}>{article.title}</Link>
        </h3>
        <p className="article-card__excerpt">{article.excerpt}</p>
        <time className="article-card__date" dateTime={article.published_at}>
          {formatDateFa(article.published_at)}
        </time>
      </div>
    </article>
  )
}
