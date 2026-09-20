import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import {
  approveReturnRequest,
  confirmManualRefund,
  getAdminReturnRequests,
  markReturnRequestReceived,
  refundReturnRequest,
  rejectReturnRequest,
} from '../../api/orders'
import { ApiError } from '../../api/errors'
import type { AdminReturnRequest } from '../../api/orders'
import { formatDateFa } from '../../lib/format'

const STATUS_LABELS: Record<string, string> = {
  requested: 'درخواست شده',
  approved: 'تایید شده',
  rejected: 'رد شده',
  received: 'مرجوعی دریافت شد',
  refund_pending: 'در انتظار بازپرداخت',
  refunded: 'وجه بازپرداخت شد',
  refund_failed: 'بازپرداخت ناموفق',
}

const STATUS_COLORS: Record<string, string> = {
  requested: 'bg-blue-100 text-blue-800',
  approved: 'bg-green-100 text-green-800',
  rejected: 'bg-red-100 text-red-800',
  received: 'bg-purple-100 text-purple-800',
  refund_pending: 'bg-yellow-100 text-yellow-800',
  refunded: 'bg-emerald-100 text-emerald-800',
  refund_failed: 'bg-red-100 text-red-800',
}

// Status text alone can read as ambiguous ("received" vs "refunded" look
// similar at a glance), so each row also gets an explicit note that spells
// out, in plain language, whether money has actually moved yet.
function statusNote(status: string): string {
  switch (status) {
    case 'received':
      return 'کالای مرجوعی دریافت شده است؛ هنوز وجهی بازپرداخت نشده است.'
    case 'refund_pending':
      return 'بازپرداخت در حال پردازش است؛ وجه هنوز بازپرداخت نشده است.'
    case 'refunded':
      return 'وجه با موفقیت بازپرداخت شده است.'
    case 'refund_failed':
      return 'بازپرداخت ناموفق بود؛ سفارش همچنان پرداخت‌شده در نظر گرفته می‌شود.'
    default:
      return ''
  }
}

export default function AdminReturnsPage() {
  const [returnRequests, setReturnRequests] = useState<AdminReturnRequest[] | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [reloadKey, setReloadKey] = useState(0)
  const [pendingActionId, setPendingActionId] = useState<number | null>(null)

  const reload = () => {
    setLoading(true)
    setError('')
    setSuccess('')
    setReloadKey((key) => key + 1)
  }

  useEffect(() => {
    let ignore = false

    getAdminReturnRequests()
      .then((data) => {
        if (!ignore) setReturnRequests(data)
      })
      .catch((err) => {
        if (!ignore) setError(err instanceof Error ? err.message : 'مشکلی در بارگذاری درخواست‌های مرجوعی پیش آمد')
      })
      .finally(() => {
        if (!ignore) setLoading(false)
      })

    return () => {
      ignore = true
    }
  }, [reloadKey])

  const patchRow = (id: number, patch: Partial<AdminReturnRequest>) => {
    setReturnRequests((prev) => {
      if (!prev) return null
      return prev.map((req) => (req.id === id ? { ...req, ...patch } : req))
    })
  }

  const runAction = async (id: number, label: string, action: () => Promise<AdminReturnRequest | void>) => {
    setPendingActionId(id)
    setError('')
    try {
      const result = await action()
      if (result) {
        patchRow(id, result)
      }
      setSuccess(label)
      setTimeout(() => setSuccess(''), 4000)
      // Refund actions can also change server-side state (e.g. status
      // moving from refund_pending to manual_review) that isn't reflected
      // in the return-request row alone, so refresh the whole list.
      reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'خطا در انجام عملیات')
    } finally {
      setPendingActionId(null)
    }
  }

  const handleApprove = (id: number) =>
    runAction(id, `درخواست مرجوعی #${id} تایید شد`, () => approveReturnRequest(id))

  const handleReject = (id: number) => {
    if (!window.confirm('آیا از رد این درخواست مرجوعی مطمئن هستید؟')) return
    return runAction(id, `درخواست مرجوعی #${id} رد شد`, () => rejectReturnRequest(id))
  }

  const handleReceived = (id: number) =>
    runAction(id, `کالای مرجوعی #${id} دریافت شد`, () => markReturnRequestReceived(id))

  const handleRefund = async (id: number) => {
    if (!window.confirm('آیا از بازپرداخت وجه این سفارش مطمئن هستید؟ این عملیات وجه را به مشتری بازمی‌گرداند.')) {
      return
    }
    setPendingActionId(id)
    setError('')
    try {
      await refundReturnRequest(id)
      setSuccess(`بازپرداخت سفارش مربوط به درخواست #${id} انجام شد`)
      setTimeout(() => setSuccess(''), 4000)
      reload()
    } catch (err) {
      if (err instanceof ApiError && err.status === 400) {
        // The generic refund action refuses manual-payment orders outright
        // and requires a separate, explicit confirmation step.
        if (
          window.confirm(
            'این سفارش با روش پرداخت دستی انجام شده و نیاز به تایید صریح بازپرداخت دستی دارد. آیا بازپرداخت دستی را تایید می‌کنید؟',
          )
        ) {
          try {
            await confirmManualRefund(id)
            setSuccess(`بازپرداخت دستی سفارش مربوط به درخواست #${id} تایید شد`)
            setTimeout(() => setSuccess(''), 4000)
            reload()
          } catch (manualErr) {
            setError(manualErr instanceof Error ? manualErr.message : 'خطا در تایید بازپرداخت دستی')
          }
        }
      } else if (err instanceof ApiError && err.status === 409) {
        // manual_review or an already-claimed refund — never show this as
        // a successful refund.
        setError('بازپرداخت نیاز به بررسی دستی دارد یا در حال پردازش توسط درخواست دیگری است. وضعیت سفارش همچنان پرداخت‌شده باقی می‌ماند.')
        reload()
      } else {
        setError(err instanceof Error ? err.message : 'خطا در بازپرداخت')
      }
    } finally {
      setPendingActionId(null)
    }
  }

  return (
    <main>
      <div className="page-header">
        <h1>مدیریت · مرجوعی‌ها</h1>
        <p className="page-subtitle">مشاهده و مدیریت درخواست‌های مرجوعی مشتریان.</p>
      </div>

      {success && (
        <p className="alert alert-success" role="alert">
          {success}
        </p>
      )}

      {loading && <p className="state-message">در حال بارگذاری درخواست‌های مرجوعی...</p>}

      {!loading && error && (
        <p className="alert alert-error" role="alert">
          {error}
        </p>
      )}

      {!loading && returnRequests && returnRequests.length === 0 && (
        <p className="empty-state">هنوز درخواست مرجوعی ثبت نشده است.</p>
      )}

      {!loading && returnRequests && returnRequests.length > 0 && (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>درخواست</th>
                <th>سفارش</th>
                <th>وضعیت</th>
                <th>دلیل</th>
                <th>تاریخ</th>
                <th>اقدامات</th>
              </tr>
            </thead>
            <tbody>
              {returnRequests.map((req) => {
                const isPending = pendingActionId === req.id
                const note = statusNote(req.status)
                return (
                  <tr key={req.id}>
                    <td>#{req.id}</td>
                    <td>
                      <Link to={`/admin/orders/${req.order_id}`}>#{req.order_id}</Link>
                    </td>
                    <td>
                      <span
                        className={`status-badge ${STATUS_COLORS[req.status] || 'bg-gray-100 text-gray-800'}`}
                        style={{ display: 'inline-block', padding: '0.25rem 0.5rem', borderRadius: '0.25rem' }}
                      >
                        {STATUS_LABELS[req.status] || req.status}
                      </span>
                      {note && <p className="field-hint">{note}</p>}
                    </td>
                    <td>{req.reason}</td>
                    <td>{formatDateFa(req.requested_at)}</td>
                    <td>
                      <div className="flex flex-col gap-1">
                        {req.status === 'requested' && (
                          <>
                            <button
                              type="button"
                              className="btn btn-success btn-sm"
                              disabled={isPending}
                              onClick={() => handleApprove(req.id)}
                            >
                              تایید
                            </button>
                            <button
                              type="button"
                              className="btn btn-danger btn-sm"
                              disabled={isPending}
                              onClick={() => handleReject(req.id)}
                            >
                              رد
                            </button>
                          </>
                        )}
                        {req.status === 'approved' && (
                          <button
                            type="button"
                            className="btn btn-secondary btn-sm"
                            disabled={isPending}
                            onClick={() => handleReceived(req.id)}
                          >
                            ثبت دریافت کالا
                          </button>
                        )}
                        {req.status === 'received' && (
                          <button
                            type="button"
                            className="btn btn-primary btn-sm"
                            disabled={isPending}
                            onClick={() => handleRefund(req.id)}
                          >
                            بازپرداخت وجه
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </main>
  )
}
