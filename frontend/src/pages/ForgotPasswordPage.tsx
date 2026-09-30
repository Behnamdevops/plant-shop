import { useState } from 'react'
import { Link } from 'react-router-dom'
import { readErrorMessage } from '../api/errors'
export default function ForgotPasswordPage() {
  const [email, setEmail] = useState(''),
    [message, setMessage] = useState(''),
    [busy, setBusy] = useState(false),
    [error, setError] = useState('')
  return (
    <main className="form-card">
      <h1>بازیابی رمز عبور</h1>
      <form
        onSubmit={async (e) => {
          e.preventDefault()
          setBusy(true)
          setError('')
          try {
            const response = await fetch('/api/v1/auth/forgot-password', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ email }),
            })
            if (!response.ok) throw new Error(await readErrorMessage(response))
            setMessage(
              'اگر حسابی با این ایمیل وجود داشته باشد، لینک بازیابی برایتان ارسال می‌شود.',
            )
          } catch (err) {
            setError(
              err instanceof Error ? err.message : 'ارسال لینک انجام نشد.',
            )
          } finally {
            setBusy(false)
          }
        }}
      >
        <label className="form-field">
          ایمیل
          <input
            type="email"
            required
            autoComplete="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </label>
        <button className="btn btn-primary" disabled={busy}>
          {busy ? 'در حال ارسال...' : 'ارسال لینک بازیابی'}
        </button>
      </form>
      {message && <p role="status">{message}</p>}
      {error && <p role="alert">{error}</p>}
      <Link to="/login">بازگشت به ورود</Link>
    </main>
  )
}
