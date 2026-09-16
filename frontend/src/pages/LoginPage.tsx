import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'

export default function LoginPage() {
  const { login } = useAuth()
  const navigate = useNavigate()

  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setError('')

    if (!email.trim() || !password) {
      setError('ایمیل و رمز عبور الزامی است')
      return
    }

    setSubmitting(true)

    try {
      await login({ email: email.trim(), password })
      navigate('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'ورود ناموفق بود')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <main>
      <div className="form-card">
        <h1>ورود</h1>

        <form onSubmit={handleSubmit}>
          <div className="form-field">
            <label htmlFor="login-email">ایمیل</label>
            <input
              id="login-email"
              type="email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              autoComplete="email"
              required
            />
          </div>

          <div className="form-field">
            <label htmlFor="login-password">رمز عبور</label>
            <input
              id="login-password"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="current-password"
              required
            />
          </div>

          {error && (
            <p className="alert alert-error" role="alert">
              {error}
            </p>
          )}

          <button type="submit" className="btn btn-primary btn-block" disabled={submitting}>
            {submitting ? 'در حال ورود...' : 'ورود'}
          </button>
        </form>

        <p className="form-footer">
          حساب کاربری ندارید؟ <Link to="/register">ثبت‌نام کنید</Link>
        </p>
      </div>
    </main>
  )
}
