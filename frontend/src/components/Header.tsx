import { useState, useEffect } from "react";
import { Link, useNavigate, useLocation } from "react-router-dom";
import { getCart } from "../api/cart";
import { useAuth } from "../hooks/useAuth";
import { storeConfig } from "../config";
import Icon from "./Icon";
export default function Header({ isPublic = true }: { isPublic?: boolean }) {
  const { user, loading, logout } = useAuth();
  const navigate = useNavigate(),
    location = useLocation();
  const [menu, setMenu] = useState<string | null>(null),
    [cartCount, setCartCount] = useState(0);
  const open = menu === location.pathname;
  useEffect(() => {
    if (loading) return;
    let active = true;
    const update = () =>
      getCart()
        .then((cart) => {
          if (active)
            setCartCount(cart.items.reduce((n, i) => n + i.quantity, 0));
        })
        .catch(() => {});
    void update();
    window.addEventListener("cart-changed", update);
    window.addEventListener("storage", update);
    return () => {
      active = false;
      window.removeEventListener("cart-changed", update);
      window.removeEventListener("storage", update);
    };
  }, [loading, user?.id]);
  const links = [
    ["/", "خانه"],
    ["/shop", "فروشگاه"],
    ["/blog", "آموزش و راهنما"],
    ["/about", "درباره ما"],
    ["/contact", "تماس با ما"],
  ];
  return (
    <>
      <div className="announcement">
        <span>محصولات تخصصی مراقبت، تغذیه و آموزش گیاهان</span>
        <Link to="/shipping">
          روش و هزینه ارسال <Icon name="arrow" size={15} />
        </Link>
      </div>
      <header className="site-header">
        <div className="header-main">
          <Link to="/" className="site-header__brand">
            <span className="brand-mark">
              <Icon name="leaf" size={30} />
            </span>
            <span>
              {storeConfig.name}
              <small>همراه سبز خانهٔ شما</small>
            </span>
          </Link>
          <form
            className="header-search"
            role="search"
            action="/shop"
            method="get"
          >
            <Icon name="search" size={20} />
            <input
              name="q"
              aria-label="جست‌وجو در فروشگاه"
              placeholder="کود، خاک یا ابزار موردنیازتان را پیدا کنید"
              type="search"
            />
            <button type="submit" className="sr-only">
              جست‌وجو
            </button>
          </form>
          <div className="header-actions">
            <Link
              to="/wishlist"
              className="header-action"
              aria-label="علاقه‌مندی‌ها"
            >
              <Icon name="heart" />
              <span>علاقه‌مندی‌ها</span>
            </Link>
            <Link
              to="/cart"
              className="header-action"
              aria-label={`سبد خرید${cartCount ? ` (${cartCount})` : ""}`}
            >
              <span className="cart-icon">
                <Icon name="bag" />
                {cartCount > 0 && (
                  <b>{new Intl.NumberFormat("fa-IR").format(cartCount)}</b>
                )}
              </span>
              <span>سبد خرید</span>
            </Link>
            {!loading && user ? (
              <details className="account-menu">
                <summary>
                  <Icon name="user" />
                  <span>حساب کاربری</span>
                </summary>
                <div>
                  <strong>{user.name}</strong>
                  <Link to="/account">حساب کاربری</Link>
                  <Link to="/orders">سفارش‌های من</Link>
                  {user.role === "admin" && <Link to="/admin">پنل مدیریت</Link>}
                  <button
                    type="button"
                    onClick={async () => {
                      await logout();
                      navigate("/");
                    }}
                  >
                    خروج از حساب
                  </button>
                </div>
              </details>
            ) : (
              <Link
                className="header-action"
                to={`/login?returnTo=${encodeURIComponent(location.pathname + location.search)}`}
              >
                <Icon name="user" />
                <span>ورود</span>
              </Link>
            )}
            <button
              className="menu-toggle"
              type="button"
              aria-label="منو"
              aria-expanded={open}
              aria-controls="site-navigation"
              onClick={() => setMenu(open ? null : location.pathname)}
            >
              <Icon name={open ? "close" : "menu"} />
            </button>
          </div>
        </div>
        {isPublic && (
          <nav
            id="site-navigation"
            aria-label="منوی اصلی"
            className={`site-header__nav ${open ? "is-open" : ""}`}
            onClick={() => setMenu(null)}
          >
            {links.map(([path, label]) => (
              <Link
                key={path}
                to={path}
                aria-current={
                  path === "/"
                    ? location.pathname === "/"
                      ? "page"
                      : undefined
                    : location.pathname.startsWith(path)
                      ? "page"
                      : undefined
                }
                className="site-header__nav-link"
              >
                {label}
              </Link>
            ))}
            <Link className="nav-feature" to="/category/bundles">
              <Icon name="leaf" size={17} /> پک‌های مراقبت
            </Link>
          </nav>
        )}
      </header>
    </>
  );
}
