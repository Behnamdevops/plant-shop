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
        Plant Shop
      </Link>

      <nav className="site-header__nav">
        <Link to="/cart">Cart</Link>
        {loading ? null : user ? (
          <>
            <Link to="/orders">Orders</Link>
            <span className="site-header__user">Hi, {user.name}</span>
            <button type="button" onClick={handleLogout}>
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
