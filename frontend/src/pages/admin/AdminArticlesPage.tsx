import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { deleteArticle, getAdminArticles, type Article } from '../../api/articles'

export default function AdminArticlesPage() {
  const [articles, setArticles] = useState<Article[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [page, setPage] = useState(1)
  const [totalPages, setTotalPages] = useState(0)

  useEffect(() => {
    getAdminArticles(page)
      .then((data) => {
        setArticles(data.items)
        setTotalPages(data.total_pages)
        setError(null)
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : 'خطا در دریافت مقالات')
      })
      .finally(() => setLoading(false))
  }, [page])

  const handleDelete = async (id: number) => {
    if (!confirm('آیا مطمئن هستید که می‌خواهید این مقاله را حذف کنید؟')) {
      return
    }

    try {
      await deleteArticle(id)
      setArticles(articles.filter(a => a.id !== id))
    } catch (err) {
      alert(err instanceof Error ? err.message : 'خطا در حذف مقاله')
    }
  }

  const formatDate = (dateStr: string) => {
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
    <div className="admin-page">
      <div className="admin-header">
        <h1>مدیریت مقالات</h1>
        <Link to="/admin/articles/new" className="btn btn-primary">
          مقاله جدید
        </Link>
      </div>

      {error && <div className="error-message">{error}</div>}

      {articles.length === 0 ? (
        <div className="empty-state">
          <p>مقاله‌ای وجود ندارد.</p>
          <Link to="/admin/articles/new" className="btn btn-primary">
            اولین مقاله را بسازید
          </Link>
        </div>
      ) : (
        <>
          <table className="admin-table">
            <thead>
              <tr>
                <th>عنوان</th>
                <th>اسلاگ</th>
                <th>دسته‌بندی</th>
                <th>وضعیت</th>
                <th>تاریخ ایجاد</th>
                <th>عملیات</th>
              </tr>
            </thead>
            <tbody>
              {articles.map((article) => (
                <tr key={article.id}>
                  <td>{article.title}</td>
                  <td>{article.slug}</td>
                  <td>{article.category?.name || '-'}</td>
                  <td>
                    <span className={`status-badge ${article.status}`}>
                      {article.status === 'published' ? 'منتشر شده' : 'پیش‌نویس'}
                    </span>
                  </td>
                  <td>{formatDate(article.created_at)}</td>
                  <td>
                    <div className="action-buttons">
                      <Link 
                        to={`/admin/articles/${article.id}/edit`} 
                        className="btn btn-sm"
                      >
                        ویرایش
                      </Link>
                      <button 
                        onClick={() => handleDelete(article.id)}
                        className="btn btn-sm btn-danger"
                      >
                        حذف
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>

          {totalPages > 1 && (
            <div className="pagination">
              <button
                disabled={page <= 1}
                onClick={() => setPage(page - 1)}
                className="pagination-btn"
              >
                قبلی
              </button>
              <span className="page-info">
                صفحه {page} از {totalPages}
              </span>
              <button
                disabled={page >= totalPages}
                onClick={() => setPage(page + 1)}
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