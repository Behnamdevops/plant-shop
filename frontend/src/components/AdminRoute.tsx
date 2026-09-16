import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'

// AdminRoute is a UX convenience only. The backend enforces the real
// authorization boundary (401/403 on every /api/v1/admin/* endpoint) — this
// component just avoids rendering admin UI for users who would immediately
// be rejected by the API.
export default function AdminRoute({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth()
  const location = useLocation()

  if (loading) {
    return (
      <main>
        <p className="state-message">Loading...</p>
      </main>
    )
  }

  if (!user) {
    return <Navigate to="/login" state={{ from: location }} replace />
  }

  if (user.role !== 'admin') {
    return (
      <main>
        <h1>Forbidden</h1>
        <p className="alert alert-error" role="alert">
          You don&apos;t have permission to view this page.
        </p>
      </main>
    )
  }

  return <>{children}</>
}
