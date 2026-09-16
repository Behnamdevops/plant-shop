import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'

export default function Header() {
  const { user, loading, logout } = useAuth()
  const navigate = useNavigate()

  const handleLogout = async () => {
    await logout()
    navigate('/')
  }

  return (
    <header className="site-header">
      <Link to="/" className="site-header__brand">
        <span className="site-header__brand-icon" aria-hidden="true">
          🌿
        </span>
        Plant Shop
      </Link>

      <nav className="site-header__nav">
        <Link to="/cart">Cart</Link>
        {/* Admin is a plain utility link, not gated by a role check: the backend
            has no admin/role concept today, so this is not real authorization. */}
        <Link to="/admin/products">Admin</Link>
        {loading ? null : user ? (
          <>
            <Link to="/orders">Orders</Link>
            <span className="site-header__user">Hi, {user.name}</span>
            <button type="button" className="btn btn-secondary btn-sm" onClick={handleLogout}>
              Log out
            </button>
          </>
        ) : (
          <>
            <Link to="/login">Log in</Link>
            <Link to="/register">Register</Link>
          </>
        )}
      </nav>
    </header>
  )
}
