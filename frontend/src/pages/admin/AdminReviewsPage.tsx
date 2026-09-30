import { useEffect, useState } from 'react'
import { readErrorMessage } from '../../api/errors'
type Review = {
  id: number
  product_id: number
  customer_name: string
  rating: number
  comment: string
  status: string
}
export default function AdminReviewsPage() {
  const [items, setItems] = useState<Review[]>([]),
    [error, setError] = useState(''),
    [loading, setLoading] = useState(true),
    [busy, setBusy] = useState<number | null>(null),
    [filter, setFilter] = useState('pending')
  useEffect(() => {
    let active = true
    fetch('/api/v1/admin/reviews')
      .then(async (response) => {
        if (!response.ok) throw new Error(await readErrorMessage(response))
        return response.json() as Promise<Review[]>
      })
      .then((value) => {
        if (active) setItems(value)
      })
      .catch((err) => {
        if (active)
          setError(err instanceof Error ? err.message : 'بارگذاری انجام نشد.')
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => {
      active = false
    }
  }, [])
  const moderate = async (id: number, status: string) => {
    setBusy(id)
    setError('')
    try {
      const response = await fetch(`/api/v1/admin/reviews/${id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ status }),
      })
      if (!response.ok) throw new Error(await readErrorMessage(response))
      setItems((old) => old.map((i) => (i.id === id ? { ...i, status } : i)))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'ذخیره انجام نشد.')
    } finally {
      setBusy(null)
    }
  }
  return (
    <main>
      <h1>مدیریت نظر مشتری</h1>
      <label>
        وضعیت
        <select value={filter} onChange={(e) => setFilter(e.target.value)}>
          <option value="">همه</option>
          <option value="pending">در انتظار بررسی</option>
          <option value="approved">منتشرشده</option>
          <option value="rejected">ردشده</option>
        </select>
      </label>
      {loading && <p>در حال بارگذاری...</p>}
      {error && <p role="alert">{error}</p>}
      {items
        .filter((i) => !filter || i.status === filter)
        .map((i) => (
          <article className="review-card" key={i.id}>
            <h2>
              محصول #{i.product_id} · {i.rating} از ۵
            </h2>
            <p>{i.customer_name}</p>
            <p>{i.comment}</p>
            <button
              className="btn btn-primary"
              disabled={busy === i.id}
              onClick={() => void moderate(i.id, 'approved')}
            >
              تأیید انتشار
            </button>{' '}
            <button
              className="btn btn-secondary"
              disabled={busy === i.id}
              onClick={() => void moderate(i.id, 'rejected')}
            >
              رد نظر
            </button>
          </article>
        ))}
    </main>
  )
}
