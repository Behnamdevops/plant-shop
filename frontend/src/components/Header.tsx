import { Link, useNavigate, useLocation } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import { storeConfig } from '../config'

type HeaderProps = {
  isPublic?: boolean
}

export default function Header({ isPublic = false }: HeaderProps) {
  const { user, loading, logout } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()

  const handleLogout = async () => {
    await logout()
    navigate('/')
  }

  const isActive = (path: string) => {
    if (path === '/' && location.pathname === '/') return true
    if (path !== '/' && location.pathname.startsWith(path)) return true
    return false
  }

  const publicLinks = [
    { path: '/', label: 'خانه' },
    { path: '/shop', label: 'فروشگاه' },
    { path: '/articles', label: 'مقالات' },
    { path: '/about', label: 'درباره ما' },
    { path: '/contact', label: 'تماس با ما' }
  ]

  return (
    <header className="site-header">
      <Link to="/" className="site-header__brand">
        <span className="site-header__brand-icon" aria-hidden="true">
          🌿
        </span>
        {storeConfig.name}
      </Link>

      <nav className="site-header__nav">
        {isPublic ? (
          <>
            {publicLinks.map((link) => (
              <Link
                key={link.path}
                to={link.path}
                className={isActive(link.path) ? 'site-header__nav-link active' : 'site-header__nav-link'}
              >
                {link.label}
              </Link>
            ))}
            <span className="site-header__nav-divider" aria-hidden="true">|</span>
          </>
        ) : null}

        <Link to="/cart" className="site-header__nav-link">
          سبد خرید
        </Link>

        {loading ? null : user ? (
          <>
            <Link to="/account" className="site-header__nav-link">
              حساب کاربری
            </Link>
            <Link to="/orders" className="site-header__nav-link">
              سفارش‌های من
            </Link>
            {user.role === 'admin' && (
              <>
                <Link to="/admin/products" className="site-header__nav-link">
                  مدیریت محصولات
                </Link>
                <Link to="/admin/articles" className="site-header__nav-link">
                  مدیریت مقالات
                </Link>
                <Link to="/admin/orders" className="site-header__nav-link">
                  مدیریت سفارش‌ها
                </Link>
              </>
            )}
            <span className="site-header__user">سلام، {user.name}</span>
            <button type="button" className="btn btn-secondary btn-sm" onClick={handleLogout}>
              خروج
            </button>
          </>
        ) : (
          <>
            <Link to="/login" className="site-header__nav-link">
              ورود
            </Link>
            <Link to="/register" className="site-header__nav-link">
              ثبت‌نام
            </Link>
          </>
        )}
      </nav>
    </header>
  )
}
