import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import { storeConfig } from '../config'

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
        {storeConfig.name}
      </Link>

      <nav className="site-header__nav">
        <Link to="/cart">سبد خرید</Link>
        {loading ? null : user ? (
          <>
            <Link to="/orders">سفارش‌های من</Link>
            {user.role === 'admin' && (
              <>
                <Link to="/admin/products">مدیریت محصولات</Link>
                <Link to="/admin/orders">مدیریت سفارش‌ها</Link>
                <Link to="/admin/coupons">کدهای تخفیف</Link>
                <Link to="/admin/payments/reconciliation">مغایرت‌گیری پرداخت‌ها</Link>
              </>
            )}
            <span className="site-header__user">سلام، {user.name}</span>
            <button type="button" className="btn btn-secondary btn-sm" onClick={handleLogout}>
              خروج
            </button>
          </>
        ) : (
          <>
            <Link to="/login">ورود</Link>
            <Link to="/register">ثبت‌نام</Link>
          </>
        )}
      </nav>
    </header>
  )
}
