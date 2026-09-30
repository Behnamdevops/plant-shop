import { useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { readErrorMessage } from '../api/errors'
export default function ResetPasswordPage() {
  const [params, setParams] = useSearchParams()
  const token = params.get('token') || ''
  const [password, setPassword] = useState(''),
    [done, setDone] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState('')
  return (
    <main className="form-card">
      <h1>انتخاب رمز جدید</h1>
      {done ? (
        <>
          <p role="status">رمز تغییر کرد. با رمز جدید وارد شوید.</p>
          <Link to="/login">ورود</Link>
        </>
      ) : (
        <form
          onSubmit={async (e) => {
            e.preventDefault()
            setError('')
            if (new TextEncoder().encode(password).length > 72) {
              setError('رمز بیش از حد طولانی است.')
              return
            }
            setBusy(true)
            try {
              const response = await fetch('/api/v1/auth/reset-password', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ token, password }),
              })
              if (!response.ok)
                throw new Error(await readErrorMessage(response))
              setDone(true)
              setParams({}, { replace: true })
            } catch (err) {
              setError(
                err instanceof Error ? err.message : 'تغییر رمز انجام نشد.',
              )
            } finally {
              setBusy(false)
            }
          }}
        >
          <label className="form-field">
            رمز جدید
            <input
              type="password"
              autoComplete="new-password"
              minLength={8}
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </label>
          <button className="btn btn-primary" disabled={busy || !token}>
            ذخیره رمز جدید
          </button>
          {error && <p role="alert">{error}</p>}
        </form>
      )}
    </main>
  )
}
