import { useEffect, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import { readErrorMessage } from '../api/errors'
type Review = {
  id: number
  customer_name: string
  rating: number
  comment: string
  created_at: string
}
type Data = { items: Review[]; count: number; average: number }
export default function ProductReviews({ productID }: { productID: number }) {
  const location = useLocation()
  const { user } = useAuth()
  const [data, setData] = useState<Data | null>(null),
    [error, setError] = useState(''),
    [message, setMessage] = useState(''),
    [rating, setRating] = useState(5),
    [comment, setComment] = useState(''),
    [busy, setBusy] = useState(false)
  useEffect(() => {
    let active = true
    fetch(`/api/v1/products/${productID}/reviews`)
      .then(async (response) => {
        if (!response.ok) throw new Error(await readErrorMessage(response))
        return response.json() as Promise<Data>
      })
      .then((value) => {
        if (active) setData(value)
      })
      .catch(() => {
        if (active) setError('بارگذاری نظرها انجام نشد.')
      })
    return () => {
      active = false
    }
  }, [productID])
  return (
    <section className="product-reviews">
      <h2>نظر خریداران</h2>
      {data && (
        <p>
          {data.count
            ? `${data.average.toFixed(1)} از ۵ · ${data.count} نظر تأییدشده`
            : 'هنوز نظری منتشر نشده است.'}
        </p>
      )}
      {data?.items.map((review) => (
        <article className="review-card" key={review.id}>
          <strong>{review.customer_name}</strong>
          <span> · {review.rating} از ۵ · خریدار تأییدشده</span>
          <p>{review.comment}</p>
        </article>
      ))}
      {user ? (
        <form
          onSubmit={async (e) => {
            e.preventDefault()
            setBusy(true)
            setError('')
            setMessage('')
            try {
              const response = await fetch(
                `/api/v1/products/${productID}/reviews`,
                {
                  method: 'POST',
                  headers: { 'Content-Type': 'application/json' },
                  body: JSON.stringify({ rating, comment }),
                },
              )
              if (!response.ok)
                throw new Error(await readErrorMessage(response))
              setMessage('نظر شما ثبت شد و پس از بررسی منتشر می‌شود.')
              setComment('')
            } catch (err) {
              setError(
                err instanceof Error ? err.message : 'ثبت نظر انجام نشد.',
              )
            } finally {
              setBusy(false)
            }
          }}
        >
          <p>
            بعد از تحویل سفارش پرداخت‌شده می‌توانید نظر ثبت کنید. ویرایش نظر
            قبلی هم دوباره بررسی می‌شود.
          </p>
          <label className="form-field">
            امتیاز
            <select
              value={rating}
              onChange={(e) => setRating(Number(e.target.value))}
            >
              {[5, 4, 3, 2, 1].map((n) => (
                <option key={n} value={n}>
                  {n} از ۵
                </option>
              ))}
            </select>
          </label>
          <label className="form-field">
            نظر شما
            <textarea
              minLength={3}
              maxLength={2000}
              required
              value={comment}
              onChange={(e) => setComment(e.target.value)}
            />
          </label>
          <button className="btn btn-primary" disabled={busy}>
            ثبت نظر
          </button>
        </form>
      ) : (
        <p>
          برای ثبت نظر{' '}
          <Link to={`/login?returnTo=${encodeURIComponent(location.pathname)}`}>
            وارد حساب شوید
          </Link>
          .
        </p>
      )}
      {error && <p role="alert">{error}</p>}
      {message && <p role="status">{message}</p>}
    </section>
  )
}
