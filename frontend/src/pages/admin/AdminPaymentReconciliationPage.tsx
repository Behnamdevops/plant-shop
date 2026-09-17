import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { ApiError } from '../../api/errors'
import { getAdminPaymentReconciliations, reconcileAdminPayment } from '../../api/paymentsAdmin'
import { formatDateFa, formatToman } from '../../lib/format'
import {
  orderStatusLabel,
  paymentAttemptStatusLabel,
  paymentMethodLabel,
  paymentStatusLabel,
  reconciliationLastOutcomeLabel,
  reconciliationReasonLabel,
} from '../../lib/labels'
import type { AdminPaymentReconciliation } from '../../types/payment'

function errorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 401) return 'نشست شما پایان یافته است. دوباره وارد شوید.'
    if (error.status === 403) return 'اجازه دسترسی به مغایرت‌گیری پرداخت‌ها را ندارید.'
    if (error.status === 404) return 'تلاش پرداخت یافت نشد.'
    if (error.status === 409) return 'وضعیت پرداخت تغییر کرده یا بررسی آن در حال انجام است. فهرست را تازه کنید.'
  }
  return 'ارتباط با سرور یا بررسی پرداخت با مشکل مواجه شد. وضعیت پرداخت را از فهرست تازه بررسی کنید.'
}

function PaymentDetails({ payment }: { payment: AdminPaymentReconciliation }) {
  return (
    <>
      <div className="order-card__header">
        <h2>تلاش پرداخت #{payment.id}</h2>
        <Link to={`/admin/orders/${payment.order_id}`}>سفارش #{payment.order_id}</Link>
      </div>
      <div className="order-card__meta">
        <span>مبلغ: {formatToman(payment.amount)}</span>
        <span>درگاه: {paymentMethodLabel(payment.provider)}</span>
        <span>واحد پول: <bdi>{payment.currency}</bdi></span>
        <span>محیط: <bdi>{payment.environment}</bdi></span>
        <span>تلاش پرداخت: {paymentAttemptStatusLabel(payment.status)}</span>
        <span>سفارش: {orderStatusLabel(payment.order_status)}</span>
        <span>پرداخت سفارش: {paymentStatusLabel(payment.payment_status)}</span>
      </div>
      <div className="order-card__meta">
        <span>علت: {reconciliationReasonLabel(payment.reason)}</span>
        <span>نیازمند مغایرت‌گیری: {payment.reconciliation_required ? 'بله' : 'خیر'}</span>
        <span>قابل بررسی مجدد: {payment.retryable ? 'بله' : 'خیر'}</span>
        <span>آخرین نتیجه: {reconciliationLastOutcomeLabel(payment.last_outcome)}</span>
        <span>آخرین بررسی: {payment.last_checked_at ? formatDateFa(payment.last_checked_at) : '—'}</span>
      </div>
      <div className="order-card__meta">
        <span>شناسه تراکنش: <bdi>{payment.authority ?? '—'}</bdi></span>
        <span>شناسه پیگیری: <bdi>{payment.ref_id ?? '—'}</bdi></span>
        <span>کد درگاه: {payment.provider_code ?? '—'}</span>
        <span>ثبت: {formatDateFa(payment.created_at)}</span>
        <span>به‌روزرسانی: {formatDateFa(payment.updated_at)}</span>
        <span>تأیید: {payment.verified_at ? formatDateFa(payment.verified_at) : '—'}</span>
      </div>
    </>
  )
}

export default function AdminPaymentReconciliationPage() {
  const [payments, setPayments] = useState<AdminPaymentReconciliation[]>([])
  const [beforeId, setBeforeId] = useState<number | undefined>()
  const [reloadKey, setReloadKey] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [submittingId, setSubmittingId] = useState<number | null>(null)
  const [actionError, setActionError] = useState('')
  const [result, setResult] = useState<AdminPaymentReconciliation | null>(null)

  useEffect(() => {
    let ignore = false
    getAdminPaymentReconciliations(beforeId)
      .then((data) => {
        if (!ignore) setPayments(data)
      })
      .catch((err: unknown) => {
        if (!ignore) setError(errorMessage(err))
      })
      .finally(() => {
        if (!ignore) setLoading(false)
      })
    return () => {
      ignore = true
    }
  }, [beforeId, reloadKey])

  const reload = (cursor: number | undefined) => {
    setLoading(true)
    setError('')
    setBeforeId(cursor)
    setReloadKey((key) => key + 1)
  }

  const reconcile = async (payment: AdminPaymentReconciliation) => {
    if (loading || submittingId !== null || !payment.retryable) return
    setSubmittingId(payment.id)
    setActionError('')
    setResult(null)
    try {
      setResult(await reconcileAdminPayment(payment.id))
    } catch (err) {
      setActionError(errorMessage(err))
    } finally {
      setSubmittingId(null)
      reload(beforeId)
    }
  }

  const busy = loading || submittingId !== null

  return (
    <main>
      <div className="page-header">
        <h1>مدیریت · مغایرت‌گیری پرداخت‌ها</h1>
        <p className="page-subtitle">هر صفحه حداکثر ۵۰ تلاش پرداخت را نمایش می‌دهد. نتیجه بررسی به معنی موفق بودن پرداخت نیست.</p>
      </div>
      <div className="order-card__meta">
        <button type="button" className="btn btn-secondary" disabled={busy} onClick={() => reload(beforeId)}>
          تازه‌سازی
        </button>
        {beforeId !== undefined && (
          <button type="button" className="btn btn-secondary" disabled={busy} onClick={() => reload(undefined)}>
            جدیدترین‌ها
          </button>
        )}
      </div>
      {actionError && <p className="alert alert-error" role="alert">{actionError}</p>}
      {result && (
        <section className="order-card" aria-live="polite" aria-label="نتیجه آخرین بررسی">
          <p>نتیجه آخرین بررسی ثبت‌شده در سرور (حتی اگر از فهرست حذف شده باشد)</p>
          <PaymentDetails payment={result} />
        </section>
      )}
      {loading && <p className="state-message">در حال بارگذاری پرداخت‌ها...</p>}
      {error && <p className="alert alert-error" role="alert">{error}</p>}
      {!loading && !error && payments.length === 0 && <p className="empty-state">پرداختی برای نمایش وجود ندارد.</p>}
      {!loading && !error && payments.map((payment) => (
        <article className="order-card" key={payment.id}>
          <PaymentDetails payment={payment} />
          <button
            type="button"
            className="btn btn-primary btn-sm"
            disabled={busy || !payment.retryable}
            onClick={() => reconcile(payment)}
          >
            {submittingId === payment.id ? 'در حال بررسی...' : 'بررسی مجدد پرداخت'}
          </button>
        </article>
      ))}
      {!loading && !error && payments.length === 50 && (
        <button type="button" className="btn btn-secondary" disabled={busy} onClick={() => reload(payments[payments.length - 1].id)}>
          قدیمی‌تر
        </button>
      )}
    </main>
  )
}
