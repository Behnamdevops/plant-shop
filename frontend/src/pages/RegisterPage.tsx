import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'

export default function RegisterPage() {
  const { register } = useAuth()
  const navigate = useNavigate()

  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setError('')

    if (!name.trim() || !email.trim() || !password) {
      setError('نام، ایمیل و رمز عبور الزامی است')
      return
    }

    if (password.length < 8) {
      setError('رمز عبور باید حداقل ۸ کاراکتر باشد')
      return
    }

    setSubmitting(true)

    try {
      await register({ name: name.trim(), email: email.trim(), password })
      navigate('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'ثبت‌نام ناموفق بود')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <main>
      <div className="form-card">
        <h1>ثبت‌نام</h1>

        <form onSubmit={handleSubmit}>
          <div className="form-field">
            <label htmlFor="register-name">نام</label>
            <input
              id="register-name"
              type="text"
              value={name}
              onChange={(event) => setName(event.target.value)}
              autoComplete="name"
              required
            />
          </div>

          <div className="form-field">
            <label htmlFor="register-email">ایمیل</label>
            <input
              id="register-email"
              type="email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              autoComplete="email"
              required
            />
          </div>

          <div className="form-field">
            <label htmlFor="register-password">رمز عبور</label>
            <input
              id="register-password"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="new-password"
              minLength={8}
              required
            />
          </div>

          {error && (
            <p className="alert alert-error" role="alert">
              {error}
            </p>
          )}

          <button type="submit" className="btn btn-primary btn-block" disabled={submitting}>
            {submitting ? 'در حال ثبت‌نام...' : 'ثبت‌نام'}
          </button>
        </form>

        <p className="form-footer">
          قبلاً حساب کاربری ساخته‌اید؟ <Link to="/login">وارد شوید</Link>
        </p>
      </div>
    </main>
  )
}
