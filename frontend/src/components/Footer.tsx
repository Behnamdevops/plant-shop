import { Link } from 'react-router-dom'
import { storeConfig } from '../config'

export default function Footer() {
  const currentYear = new Date().getFullYear()

  return (
    <footer className="site-footer">
      <div className="site-footer__container">
        {/* Brand & Social */}
        <div className="site-footer__brand">
          <Link to="/" className="site-footer__brand-link">
            <span className="site-footer__brand-icon" aria-hidden="true">
              🌿
            </span>
            <span className="site-footer__brand-name">{storeConfig.name}</span>
          </Link>
          <p className="site-footer__brand-desc">
            دنیای سبز گیاهان برای زندگی سالم‌تر
          </p>
        </div>

        {/* Links */}
        <div className="site-footer__links">
          {/* Shop */}
          <div className="site-footer__column">
            <h3 className="site-footer__column-title">فروشگاه</h3>
            <ul className="site-footer__column-list">
              <li>
                <Link to="/shop">همه محصولات</Link>
              </li>
              <li>
                <Link to="/shop?category=1">گیاهات اطراف خانه</Link>
              </li>
              <li>
                <Link to="/shop?category=2">گیاهات داخل خانه</Link>
              </li>
              <li>
                <Link to="/shop?category=3">گل‌ها</Link>
              </li>
            </ul>
          </div>

          {/* Education */}
          <div className="site-footer__column">
            <h3 className="site-footer__column-title">آموزش و محتوا</h3>
            <ul className="site-footer__column-list">
              <li>
                <Link to="/articles">مقالات و آموزش‌ها</Link>
              </li>
              <li>
                <Link to="/shipping">ارسال و حمل</Link>
              </li>
              <li>
                <Link to="/faq">سوالات متداول</Link>
              </li>
              <li>
                <Link to="/about">درباره ما</Link>
              </li>
            </ul>
          </div>

          {/* Customer Help */}
          <div className="site-footer__column">
            <h3 className="site-footer__column-title">کمک و پشتیبانی</h3>
            <ul className="site-footer__column-list">
              <li>
                <Link to="/contact">تماس با ما</Link>
              </li>
              <li>
                <Link to="/returns">سیاست بازگرداندن</Link>
              </li>
              <li>
                <Link to="/privacy">حریم خصوصی</Link>
              </li>
              <li>
                <Link to="/terms">شرایط و قوانین</Link>
              </li>
            </ul>
          </div>

          {/* Legal */}
          <div className="site-footer__column">
            <h3 className="site-footer__column-title">قوانین</h3>
            <ul className="site-footer__column-list">
              <li>
                <Link to="/privacy">حریم خصوصی</Link>
              </li>
              <li>
                <Link to="/terms">شرایط و قوانین</Link>
              </li>
              <li>
                <Link to="/returns">سیاست بازگرداندن</Link>
              </li>
            </ul>
          </div>
        </div>
      </div>

      {/* Bottom */}
      <div className="site-footer__bottom">
        <p className="site-footer__copyright">
          © {currentYear} {storeConfig.name}. تمامی حقوق محفوظ است.
        </p>
        <p className="site-footer__credits">
          طراحی شده برای دنیای سبز
        </p>
      </div>
    </footer>
  )
}
